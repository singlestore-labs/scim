package runscope

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunnerExecutesRequestAssertionsAndExtractions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filter"); got != `userName eq "alice@example.com"` {
			t.Errorf("filter = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"totalResults":1,"Resources":[{"id":"user-1"}]}`)
	}))
	defer server.Close()

	suite, err := Load(strings.NewReader(`{
		"name": "example",
		"steps": [{
			"step_type": "request",
			"method": "GET",
			"url": "{{base}}/Users?filter=userName eq \"{{username}}\"",
			"assertions": [
				{"source":"response_status","comparison":"equal_number","value":"200"},
				{"source":"response_json","comparison":"equal_number","property":"totalResults","value":"1"},
				{"source":"response_json","comparison":"not_empty","property":"Resources[0].id"}
			],
			"variables": [{"source":"response_json","name":"id","property":"Resources[0].id"}]
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	variables := Variables{"base": server.URL, "username": "alice@example.com"}
	if err := (Runner{SkipPauses: true}).Run(context.Background(), suite, variables); err != nil {
		t.Fatal(err)
	}
	if variables["id"] != "user-1" {
		t.Fatalf("id = %q", variables["id"])
	}
}

func TestRunnerRejectsUnsupportedStep(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Suite{
		Steps: []Step{{Type: "condition"}},
	}, Variables{})
	if err == nil || !strings.Contains(err.Error(), `unsupported step type "condition"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
