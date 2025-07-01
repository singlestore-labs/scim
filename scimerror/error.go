package scimerror

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/memsql/errors"
	"github.com/muir/nvelope"
)

// fake error to write status code
var NoErrorReturning201 = nvelope.ReturnCode(errors.String(""), 201)

// Needs:
// 1. internal error.. and pass along not been changed
// 2. new error with assigned scimError type
// 3. already scimError, should not been changed as well
// 4. internal distinguish 'not-found'

// Design:
//  default error as internal error, ErrNotFound as special internal error
//  scimError as scimError
//

// all SCIMErrorType is 404
type SCIMErrorType string

const (
	InvalidFilter   SCIMErrorType = "invalidFilter"
	TooMany         SCIMErrorType = "tooMany"
	UniquenessError SCIMErrorType = "uniqueness"
	MutabilityError SCIMErrorType = "mutability"
	InvalidSyntax   SCIMErrorType = "invalidSyntax"
	InvalidPath     SCIMErrorType = "invalidPath"
	NoTarget        SCIMErrorType = "noTarget"
	InvalidValue    SCIMErrorType = "invalidValue"
	InvalidVers     SCIMErrorType = "invalidVers"
	Sensitive       SCIMErrorType = "sensitive"
)

// internal special error
const ErrNotFound errors.String = "not found"

type SCIMError struct {
	Detail   error `json:"detail"`
	Status   int   `json:"status"`
	scimType SCIMErrorType
}

var _ error = &SCIMError{}

func (se *SCIMError) Error() string {
	return fmt.Sprintf("%s, %d, %s", se.scimType, se.Status, se.Detail.Error())
}

func (se *SCIMError) Is(target error) bool {
	return errors.Is(se.Detail, target)
}

func NewSCIMErr(status int, e error) error {
	if e == nil {
		return nil
	}
	return &SCIMError{
		Detail: e,
		Status: status,
	}
}

// NewBadRequestSCIMErr returns SCIMErr with SCIMErrorType and fixed bad request status code
// https://datatracker.ietf.org/doc/html/rfc7644#section-3.12
func NewBadRequestSCIMErr(t SCIMErrorType, e error) error {
	if e == nil {
		return nil
	}
	return &SCIMError{
		Detail:   e,
		Status:   http.StatusBadRequest,
		scimType: t,
	}
}

func (se SCIMError) MarshalSCIM() ([]byte, error) {
	withSchema := struct {
		Schemas  []string `json:"schemas"`
		SCIMType string   `json:"scimType,omitempty"`
		Detail   string   `json:"detail"`
		Status   int      `json:"status"`
	}{
		Schemas:  []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		SCIMType: string(se.scimType),
		Detail:   se.Detail.Error(),
		Status:   se.Status,
	}
	return json.Marshal(withSchema)
}
