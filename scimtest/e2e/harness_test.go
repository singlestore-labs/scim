package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/muir/nchi"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimtest"
	"github.com/singlestore-labs/scim/util"
	"github.com/stretchr/testify/require"
)

const (
	testBasePath = "/scim"
	patchSchema  = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	userSchema   = "urn:ietf:params:scim:schemas:core:2.0:User"
	groupSchema  = "urn:ietf:params:scim:schemas:core:2.0:Group"
)

// Harness keeps HTTP mechanics and the stable SCIM endpoint paths out of tests.
// Use Result directly for negative tests and the Must methods for successful flows.
type Harness struct {
	t       *testing.T
	handler http.Handler
	Data    *DataFactory
}

func NewHarness(t *testing.T) *Harness {
	t.Helper()

	router := nchi.NewRouter()
	server := scimtest.NewServer(util.Trace(t), scimtest.NewInMemStorage())
	router.Route(testBasePath, scimprotocol.SCIMRouter(util.Trace(t), server))

	return &Harness{
		t:       t,
		handler: router,
		Data:    NewDataFactory(t.Name()),
	}
}

// Result preserves the HTTP response for negative and protocol-level assertions.
type Result struct {
	t      *testing.T
	Status int
	Header http.Header
	Body   []byte
}

type SCIMError struct {
	Schemas  []string `json:"schemas"`
	SCIMType string   `json:"scimType"`
	Detail   string   `json:"detail"`
	Status   int      `json:"status"`
}

type User struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	UserName    string   `json:"userName"`
	DisplayName string   `json:"displayName"`
	Active      bool     `json:"active"`
	Name        Name     `json:"name"`
	Emails      []Email  `json:"emails"`
}

type Name struct {
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

type Email struct {
	Value   string `json:"value"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
}

type UserList struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	Resources    []User   `json:"Resources"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
}

type Group struct {
	Schemas     []string      `json:"schemas"`
	ID          string        `json:"id"`
	DisplayName string        `json:"displayName"`
	ExternalID  string        `json:"externalId"`
	Members     []GroupMember `json:"members"`
}

type GroupMember struct {
	Value   string `json:"value"`
	Display string `json:"display"`
	Ref     string `json:"$ref"`
	Type    string `json:"type"`
}

type GroupList struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	Resources    []Group  `json:"Resources"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
}

type UserInput struct {
	Schemas     []string `json:"schemas"`
	UserName    string   `json:"userName"`
	DisplayName string   `json:"displayName,omitempty"`
	Active      bool     `json:"active"`
	Name        Name     `json:"name,omitempty"`
	Emails      []Email  `json:"emails,omitempty"`
}

type GroupInput struct {
	Schemas     []string      `json:"schemas"`
	DisplayName string        `json:"displayName"`
	ExternalID  string        `json:"externalId,omitempty"`
	Members     []GroupMember `json:"members,omitempty"`
}

type ListOpts struct {
	Filter             string
	StartIndex         int
	Count              *int
	Attributes         []string
	ExcludedAttributes []string
}

type PatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

type patchDocument struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"`
}

func (h *Harness) CreateUser(body any) Result {
	return h.do(http.MethodPost, "/Users", nil, body)
}

func (h *Harness) GetUser(id string) Result {
	return h.do(http.MethodGet, "/Users/"+url.PathEscape(id), nil, nil)
}

func (h *Harness) ListUsers(opts ListOpts) Result {
	return h.do(http.MethodGet, "/Users", listQuery(opts), nil)
}

func (h *Harness) ReplaceUser(id string, body any) Result {
	return h.do(http.MethodPut, "/Users/"+url.PathEscape(id), nil, body)
}

// PatchUser accepts either []PatchOp, one PatchOp, json.RawMessage, or any
// JSON-marshalable complete PATCH document.
func (h *Harness) PatchUser(id string, body any) Result {
	switch value := body.(type) {
	case PatchOp:
		body = patchDocument{Schemas: []string{patchSchema}, Operations: []PatchOp{value}}
	case []PatchOp:
		body = patchDocument{Schemas: []string{patchSchema}, Operations: value}
	}
	return h.do(http.MethodPatch, "/Users/"+url.PathEscape(id), nil, body)
}

func (h *Harness) DeleteUser(id string) Result {
	return h.do(http.MethodDelete, "/Users/"+url.PathEscape(id), nil, nil)
}

func (h *Harness) CreateGroup(body any) Result {
	return h.do(http.MethodPost, "/Groups", nil, body)
}

func (h *Harness) GetGroup(id string) Result {
	return h.do(http.MethodGet, "/Groups/"+url.PathEscape(id), nil, nil)
}

func (h *Harness) ListGroups(opts ListOpts) Result {
	return h.do(http.MethodGet, "/Groups", listQuery(opts), nil)
}

func (h *Harness) ReplaceGroup(id string, body any) Result {
	return h.do(http.MethodPut, "/Groups/"+url.PathEscape(id), nil, body)
}

func (h *Harness) PatchGroup(id string, body any) Result {
	switch value := body.(type) {
	case PatchOp:
		body = patchDocument{Schemas: []string{patchSchema}, Operations: []PatchOp{value}}
	case []PatchOp:
		body = patchDocument{Schemas: []string{patchSchema}, Operations: value}
	}
	return h.do(http.MethodPatch, "/Groups/"+url.PathEscape(id), nil, body)
}

func (h *Harness) DeleteGroup(id string) Result {
	return h.do(http.MethodDelete, "/Groups/"+url.PathEscape(id), nil, nil)
}

func (h *Harness) GetServiceProviderConfig() Result {
	return h.do(http.MethodGet, "/ServiceProviderConfig", nil, nil)
}

func (h *Harness) GetSchemas() Result {
	return h.do(http.MethodGet, "/Schemas", nil, nil)
}

func (h *Harness) GetResourceTypes() Result {
	return h.do(http.MethodGet, "/ResourceTypes", nil, nil)
}

func (h *Harness) MustCreateUser(body any) User {
	h.t.Helper()
	result := h.CreateUser(body).RequireStatus(http.StatusCreated)
	user := result.User()
	require.NotEmpty(h.t, user.ID, result.String())
	return user
}

func (h *Harness) MustGetUser(id string) User {
	h.t.Helper()
	return h.GetUser(id).RequireStatus(http.StatusOK).User()
}

func (h *Harness) MustListUsers(opts ListOpts) UserList {
	h.t.Helper()
	list := h.ListUsers(opts).
		RequireStatus(http.StatusOK).
		RequireJSONContains(map[string]any{
			"schemas": []any{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		}).
		UserList()
	require.Len(h.t, list.Resources, list.ItemsPerPage)
	return list
}

func (h *Harness) MustPatchUser(id string, body any) {
	h.t.Helper()
	h.PatchUser(id, body).RequireSuccess()
}

func (h *Harness) PatchThenGetUser(id string, body any) User {
	h.t.Helper()
	h.MustPatchUser(id, body)
	return h.MustGetUser(id)
}

func (h *Harness) MustDeleteUser(id string) {
	h.t.Helper()
	h.DeleteUser(id).RequireStatus(http.StatusNoContent)
}

func (h *Harness) MustCreateGroup(body any) Group {
	h.t.Helper()
	result := h.CreateGroup(body).RequireStatus(http.StatusCreated)
	group := result.Group()
	require.NotEmpty(h.t, group.ID, result.String())
	return group
}

func (h *Harness) MustGetGroup(id string) Group {
	h.t.Helper()
	return h.GetGroup(id).RequireStatus(http.StatusOK).Group()
}

func (h *Harness) MustListGroups(opts ListOpts) GroupList {
	h.t.Helper()
	list := h.ListGroups(opts).
		RequireStatus(http.StatusOK).
		RequireJSONContains(map[string]any{
			"schemas": []any{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		}).
		GroupList()
	require.Len(h.t, list.Resources, list.ItemsPerPage)
	return list
}

func (h *Harness) MustPatchGroup(id string, body any) {
	h.t.Helper()
	h.PatchGroup(id, body).RequireSuccess()
}

func (h *Harness) PatchThenGetGroup(id string, body any) Group {
	h.t.Helper()
	h.MustPatchGroup(id, body)
	return h.MustGetGroup(id)
}

func (h *Harness) MustDeleteGroup(id string) {
	h.t.Helper()
	h.DeleteGroup(id).RequireStatus(http.StatusNoContent)
}

func (h *Harness) do(method, path string, query url.Values, body any) Result {
	h.t.Helper()

	var reader io.Reader
	if body != nil {
		var encoded []byte
		var err error
		switch value := body.(type) {
		case json.RawMessage:
			encoded = value
		case []byte:
			encoded = value
		case string:
			encoded = []byte(value)
		default:
			encoded, err = json.Marshal(body)
			require.NoError(h.t, err)
		}
		reader = bytes.NewReader(encoded)
	}

	target := testBasePath + path
	if len(query) != 0 {
		target += "?" + query.Encode()
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Authorization", "Bearer "+scimtest.DummyToken)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	h.handler.ServeHTTP(response, request)

	responseBody, err := io.ReadAll(response.Body)
	require.NoError(h.t, err)
	return Result{t: h.t, Status: response.Code, Header: response.Header(), Body: responseBody}
}

func listQuery(opts ListOpts) url.Values {
	query := make(url.Values)
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	if opts.StartIndex != 0 {
		query.Set("startIndex", strconv.Itoa(opts.StartIndex))
	}
	if opts.Count != nil {
		query.Set("count", strconv.Itoa(*opts.Count))
	}
	if len(opts.Attributes) != 0 {
		query.Set("attributes", strings.Join(opts.Attributes, ","))
	}
	if len(opts.ExcludedAttributes) != 0 {
		query.Set("excludedAttributes", strings.Join(opts.ExcludedAttributes, ","))
	}
	return query
}

func (r Result) String() string {
	return fmt.Sprintf("HTTP %d: %s", r.Status, r.Body)
}

func decode[T any](t *testing.T, result Result) T {
	t.Helper()
	var value T
	require.NoError(t, json.Unmarshal(result.Body, &value), result.String())
	return value
}

// DataFactory generates readable values that remain unique under parallel use.
type DataFactory struct {
	prefix string
	next   atomic.Uint64
	mu     sync.Mutex
	issued map[string]struct{}
}

func NewDataFactory(testName string) *DataFactory {
	replacer := strings.NewReplacer("/", "-", "_", "-", " ", "-")
	prefix := strings.ToLower(replacer.Replace(testName))
	return &DataFactory{prefix: prefix, issued: make(map[string]struct{})}
}

func (f *DataFactory) User() UserInput {
	suffix := f.unique("user")
	return UserInput{
		Schemas:     []string{userSchema},
		UserName:    suffix + "@example.test",
		DisplayName: "Test User " + suffix,
		Active:      true,
		Name:        Name{GivenName: "Test", FamilyName: suffix},
		Emails:      []Email{{Value: suffix + "@example.test", Type: "work", Primary: true}},
	}
}

func (f *DataFactory) Group() GroupInput {
	suffix := f.unique("group")
	return GroupInput{
		Schemas:     []string{groupSchema},
		DisplayName: "Test Group " + suffix,
		ExternalID:  suffix,
	}
}

func (f *DataFactory) unique(kind string) string {
	for {
		value := fmt.Sprintf("e2e-%s-%s-%d", f.prefix, kind, f.next.Add(1))
		f.mu.Lock()
		_, exists := f.issued[value]
		if !exists {
			f.issued[value] = struct{}{}
		}
		f.mu.Unlock()
		if !exists {
			return value
		}
	}
}

func intPointer(value int) *int {
	return &value
}
