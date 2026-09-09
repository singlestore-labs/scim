// Package runscope executes the request steps in a BlazeMeter API Monitoring
// (formerly Runscope) JSON export.
package runscope

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type Suite struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
}

type Step struct {
	Type          string              `json:"step_type"`
	Skipped       bool                `json:"skipped"`
	Note          string              `json:"note"`
	Duration      float64             `json:"duration"`
	Method        string              `json:"method"`
	URL           string              `json:"url"`
	Body          string              `json:"body"`
	Headers       map[string][]string `json:"headers"`
	Assertions    []Assertion         `json:"assertions"`
	Variables     []Extraction        `json:"variables"`
	Scripts       []string            `json:"scripts"`
	BeforeScripts []string            `json:"before_scripts"`
}

type Assertion struct {
	Comparison string          `json:"comparison"`
	Source     string          `json:"source"`
	Property   string          `json:"property"`
	Value      json.RawMessage `json:"value"`
}

type Extraction struct {
	Source   string `json:"source"`
	Name     string `json:"name"`
	Property string `json:"property"`
}

type Variables map[string]string

type Response struct {
	StatusCode int
	Body       []byte
	Duration   time.Duration
}

type PostResponseScriptHandler func(Step, Response, Variables) error

type Runner struct {
	Client                    *http.Client
	SkipPauses                bool
	PostResponseScriptHandler PostResponseScriptHandler
	Logf                      func(string, ...any)
}

func Load(r io.Reader) (Suite, error) {
	var suite Suite
	if err := json.NewDecoder(r).Decode(&suite); err != nil {
		return Suite{}, fmt.Errorf("decode Runscope export: %w", err)
	}
	if len(suite.Steps) == 0 {
		return Suite{}, fmt.Errorf("Runscope export %q has no steps", suite.Name)
	}
	return suite, nil
}

func (r Runner) Run(ctx context.Context, suite Suite, variables Variables) error {
	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	for index, step := range suite.Steps {
		if step.Skipped {
			continue
		}
		label := step.Note
		if label == "" {
			label = fmt.Sprintf("step %d", index+1)
		}

		switch step.Type {
		case "pause":
			if !r.SkipPauses {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(step.Duration * float64(time.Second))):
				}
			}
		case "request":
			if r.Logf != nil {
				r.Logf("%s", label)
			}
			if err := r.runRequest(ctx, client, step, variables); err != nil {
				return fmt.Errorf("%s: %w", label, err)
			}
		default:
			return fmt.Errorf("%s: unsupported step type %q", label, step.Type)
		}
	}
	return nil
}

func (r Runner) runRequest(ctx context.Context, client *http.Client, step Step, variables Variables) error {
	if hasContent(step.BeforeScripts) {
		return fmt.Errorf("before_scripts are not supported")
	}
	url, err := substitute(step.URL, variables)
	if err != nil {
		return err
	}
	url, err = normalizeURL(url)
	if err != nil {
		return err
	}
	body, err := substitute(step.Body, variables)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, step.Method, url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	for name, values := range step.Headers {
		for _, value := range values {
			value, err = substitute(value, variables)
			if err != nil {
				return fmt.Errorf("header %s: %w", name, err)
			}
			request.Header.Add(name, value)
		}
	}

	started := time.Now()
	response, err := client.Do(request)
	elapsed := time.Since(started)
	if err != nil {
		return fmt.Errorf("%s %s: %w", step.Method, url, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	result := Response{StatusCode: response.StatusCode, Body: responseBody, Duration: elapsed}

	for _, assertion := range step.Assertions {
		if err := checkAssertion(assertion, result, variables); err != nil {
			return fmt.Errorf("assertion %s %s: %w; response=%s",
				assertion.Source, assertion.Comparison, err, responseBody)
		}
	}
	for _, extraction := range step.Variables {
		if extraction.Source != "response_json" {
			return fmt.Errorf("unsupported variable source %q", extraction.Source)
		}
		value := jsonValue(responseBody, extraction.Property)
		if !value.Exists() {
			return fmt.Errorf("extract %q: property %q not found", extraction.Name, extraction.Property)
		}
		variables[extraction.Name] = value.String()
	}
	if hasContent(step.Scripts) {
		if r.PostResponseScriptHandler == nil {
			return fmt.Errorf("post-response scripts require a PostResponseScriptHandler")
		}
		if err := r.PostResponseScriptHandler(step, result, variables); err != nil {
			return fmt.Errorf("post-response script: %w", err)
		}
	}
	return nil
}

func checkAssertion(assertion Assertion, response Response, variables Variables) error {
	expected, err := rawValue(assertion.Value)
	if err != nil {
		return err
	}
	expected, err = substitute(expected, variables)
	if err != nil {
		return err
	}

	switch assertion.Source {
	case "response_status":
		if assertion.Comparison != "equal_number" {
			return fmt.Errorf("unsupported response_status comparison %q", assertion.Comparison)
		}
		return compareNumber(float64(response.StatusCode), expected)
	case "response_time":
		if assertion.Comparison != "is_less_than" {
			return fmt.Errorf("unsupported response_time comparison %q", assertion.Comparison)
		}
		limit, err := strconv.ParseFloat(expected, 64)
		if err != nil {
			return fmt.Errorf("invalid number %q", expected)
		}
		actual := float64(response.Duration) / float64(time.Millisecond)
		if actual >= limit {
			return fmt.Errorf("got %.2fms, want less than %.2fms", actual, limit)
		}
		return nil
	case "response_json":
		return compareJSON(jsonValue(response.Body, assertion.Property), assertion.Comparison, expected)
	default:
		return fmt.Errorf("unsupported assertion source %q", assertion.Source)
	}
}

func compareJSON(actual gjson.Result, comparison, expected string) error {
	switch comparison {
	case "not_empty":
		if !actual.Exists() || actual.Type == gjson.Null || actual.String() == "" ||
			(actual.IsArray() && len(actual.Array()) == 0) {
			return fmt.Errorf("value is empty or missing")
		}
	case "is_a_number":
		if actual.Type != gjson.Number {
			return fmt.Errorf("got %s, want a number", actual.Raw)
		}
	case "equal":
		if actual.String() != expected {
			return fmt.Errorf("got %q, want %q", actual.String(), expected)
		}
	case "equal_number":
		if actual.Type != gjson.Number {
			return fmt.Errorf("got %s, want a number", actual.Raw)
		}
		return compareNumber(actual.Float(), expected)
	case "has_value", "contains":
		if actual.IsArray() {
			for _, item := range actual.Array() {
				if item.String() == expected {
					return nil
				}
			}
			return fmt.Errorf("%s does not contain %q", actual.Raw, expected)
		}
		if !strings.Contains(actual.String(), expected) {
			return fmt.Errorf("%q does not contain %q", actual.String(), expected)
		}
	default:
		return fmt.Errorf("unsupported response_json comparison %q", comparison)
	}
	return nil
}

func compareNumber(actual float64, expected string) error {
	want, err := strconv.ParseFloat(expected, 64)
	if err != nil {
		return fmt.Errorf("invalid number %q", expected)
	}
	if math.Abs(actual-want) > 1e-9 {
		return fmt.Errorf("got %v, want %v", actual, want)
	}
	return nil
}

var (
	templatePattern = regexp.MustCompile(`\{\{([^{}]+)\}\}`)
	indexPattern    = regexp.MustCompile(`\[(\d+)\]`)
)

func substitute(input string, variables Variables) (string, error) {
	var missing string
	output := templatePattern.ReplaceAllStringFunc(input, func(token string) string {
		name := strings.TrimSpace(token[2 : len(token)-2])
		value, ok := variables[name]
		if !ok {
			missing = name
			return token
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("variable %q is not defined", missing)
	}
	return output, nil
}

func jsonValue(body []byte, property string) gjson.Result {
	if property == "" {
		return gjson.ParseBytes(body)
	}
	return gjson.GetBytes(body, indexPattern.ReplaceAllString(property, ".$1"))
}

func normalizeURL(input string) (string, error) {
	parsed, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("parse URL query: %w", err)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func rawValue(value json.RawMessage) (string, error) {
	if len(value) == 0 || string(value) == "null" {
		return "", nil
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return "", fmt.Errorf("decode assertion value: %w", err)
	}
	return fmt.Sprint(decoded), nil
}

func hasContent(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
