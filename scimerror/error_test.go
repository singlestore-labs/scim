package scimerror

import (
	"errors"
	"testing"

	"github.com/tidwall/gjson"
)

func TestSCIMErrorMarshalsStatusAsString(t *testing.T) {
	body, err := (SCIMError{Detail: errors.New("unauthorized"), Status: 401}).MarshalSCIM()
	if err != nil {
		t.Fatal(err)
	}
	status := gjson.GetBytes(body, "status")
	if status.Type != gjson.String || status.String() != "401" {
		t.Fatalf("status = %s, want string %q", status.Raw, "401")
	}
}
