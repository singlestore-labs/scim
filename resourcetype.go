package scimprotocol

import (
	"encoding/json"
	"reflect"
)

type Resource interface { // like User and Group
	isSCIMResource()
}

type SCIMResourceMarker struct{}

func (SCIMResourceMarker) isSCIMResource() {}

type MultiValueElement interface {
	isEqual(input MultiValueElement) bool
}

type SchemaExtention struct {
	Schema   string `json:"schema"`
	Required bool   `json:"required"`
}
type ResourceTypeMeta struct {
	Location     string `json:"location"`
	ResourceType string `json:"resourceType"`
}
type ResourceType struct {
	ID                 string           `json:"id,omitempty"`
	Name               string           `json:"name"`
	Endpoint           string           `json:"endpoint"` // "/Users" or "/Groups"
	Description        string           `json:"description,omitempty"`
	ResourceObjectType reflect.Type     `json:"-"`
	Meta               ResourceTypeMeta `json:"meta"`
}

func (r ResourceType) MarshalSCIM() ([]byte, error) {
	schema, extensions, err := GetSchemaURIFromResource(r.ResourceObjectType, nil)
	if err != nil {
		return nil, err
	}
	r.Meta.ResourceType = "ResourceType"
	withSchema := struct {
		Schemas []string `json:"schemas"`
		ResourceType
		Schema          string            `json:"schema"`
		SchemaExtension []SchemaExtention `json:"schemaExtension,omitempty"`
	}{
		Schemas:         []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
		ResourceType:    r,
		Schema:          schema,
		SchemaExtension: extensions,
	}
	return json.Marshal(withSchema)
}

type ListResponse[T any] struct {
	Resources    []T
	StartIndex   int
	ItemsPerPage int // return actual items number per page
	TotalResults int
}

var _ SCIMMarshaler = ListResponse[json.RawMessage]{}

func (l ListResponse[T]) MarshalSCIM() ([]byte, error) {
	resources, err := Marshal(l.Resources)
	if err != nil {
		return nil, err
	}

	withSchema := struct {
		Schemas      []string        `json:"schemas"`
		TotalResults int             `json:"totalResults"` // ,returned=always
		Resources    json.RawMessage `json:"Resources"`
		StartIndex   int             `json:"startIndex"`
		ItemsPerPage int             `json:"itemsPerPage"`
	}{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: l.TotalResults,
		Resources:    resources,
		StartIndex:   l.StartIndex,
		ItemsPerPage: l.ItemsPerPage,
	}
	return json.Marshal(withSchema)
}
