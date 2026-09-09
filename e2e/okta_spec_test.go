package e2e

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/singlestore-labs/scim/e2e/runscope"
	"github.com/singlestore-labs/scim/scimtest"
	"github.com/tidwall/gjson"
)

// Source: https://developer.okta.com/standards/SCIM/SCIMFiles/Okta-SCIM-20-SPEC-Test.json
//
//go:embed testdata/Okta-SCIM-20-SPEC-Test.json
var oktaSpec []byte

func TestOktaSCIM20Spec(t *testing.T) {
	suiteInput := io.Reader(strings.NewReader(string(oktaSpec)))
	if path := os.Getenv("RUNSCOPE_SUITE"); path != "" {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		suiteInput = file
	}
	suite, err := runscope.Load(suiteInput)
	if err != nil {
		t.Fatal(err)
	}

	baseURL := os.Getenv("SCIM_BASE_URL")
	if baseURL == "" {
		storage := scimtest.NewInMemStorage()
		scimtest.SeedExampleData(storage)
		server := httptest.NewServer(scimtest.NewMux(nil, storage, scimtest.DefaultPrefix))
		t.Cleanup(server.Close)
		baseURL = server.URL + scimtest.DefaultPrefix
	}
	auth := os.Getenv("SCIM_AUTH")
	if auth == "" {
		auth = "Bearer " + scimtest.DummyToken
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	variables := runscope.Variables{
		"SCIMBaseURL":            strings.TrimRight(baseURL, "/"),
		"auth":                   auth,
		"InvalidUserEmail":       "missing-" + suffix + "@example.com",
		"UserIdThatDoesNotExist": "010101001010101011001010101011",
		"randomGivenName":        "Runscope" + suffix,
		"randomFamilyName":       "Tester" + suffix,
		"randomEmail":            "runscope-" + suffix + "@example.com",
	}
	variables["randomUsername"] = variables["randomEmail"]
	variables["randomUsernameCaps"] = strings.ToUpper(variables["randomUsername"])
	if input := os.Getenv("RUNSCOPE_VARIABLES"); input != "" {
		var overrides map[string]string
		if err := json.Unmarshal([]byte(input), &overrides); err != nil {
			t.Fatalf("parse RUNSCOPE_VARIABLES: %v", err)
		}
		for name, value := range overrides {
			variables[name] = value
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runner := runscope.Runner{
		Client:                    &http.Client{Timeout: 10 * time.Second},
		SkipPauses:                true,
		PostResponseScriptHandler: validateOktaGroupScript,
		Logf:                      t.Logf,
	}
	if err := runner.Run(ctx, suite, variables); err != nil {
		t.Fatal(err)
	}
}

func validateOktaGroupScript(step runscope.Step, response runscope.Response, _ runscope.Variables) error {
	if len(step.Scripts) != 1 ||
		!strings.Contains(step.Scripts[0], "No Groups found in the endpoint") {
		return fmt.Errorf("unsupported Okta post-response script")
	}
	total := gjson.GetBytes(response.Body, "totalResults")
	resources := gjson.GetBytes(response.Body, "Resources")
	if total.Int() < 1 {
		return fmt.Errorf("no groups found in the endpoint")
	}
	if !resources.IsArray() {
		return fmt.Errorf("Resources is not an array")
	}
	return nil
}
