package scimprotocol

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/memsql/errors"
	"github.com/singlestore-labs/scim/scimtag"
	"github.com/singlestore-labs/scim/util"
)

// Schema's name and description is option in rfc and not fit in scim tag, so not include for now.
type Schema struct {
	ID         string            `json:"id"`
	Attributes []AttributeSchema `json:"attributes"`
}

var _ SCIMMarshaler = Schema{}

func (s Schema) MarshalSCIM() ([]byte, error) {
	return json.Marshal(s)
}

type AttributeSchema struct {
	Name            string             `json:"name"`
	Type            string             `json:"type"`
	MultiValued     bool               `json:"multiValued"`
	Required        bool               `json:"required"`
	CaseExact       *bool              `json:"caseExact,omitempty"`
	Mutability      scimtag.Mutability `json:"mutability"`
	Returned        scimtag.Returned   `json:"returned"`
	Uniqueness      *string            `json:"uniqueness,omitempty"`
	CanonicalValues *[]string          `json:"canonicalValues,omitempty"`
	ReferenceTypes  []string           `json:"referenceTypes,omitempty"`
	SubAttributes   []AttributeSchema  `json:"subAttributes,omitempty"`
}

// resource is User or Groups or... , that User contains Core User attributes group and Extensions attributes group
func GetResourceSchema(resourceT reflect.Type) (schemas []Schema, err error) {
	subAttrCharacs, err := scimtag.GetSCIMCharacsInSubAttributes(resourceT)
	if err != nil {
		return nil, err
	}
	for _, v := range subAttrCharacs {
		if URIPattenRegexp.MatchString(v.Tag.Name) {
			var schema Schema
			schema.ID = v.Tag.Name
			schema.Attributes, err = getAttributeGroupSchema(v.Field.Type)
			schemas = append(schemas, schema)
		}
	}
	return
}

// getAttributeGroupSchema's input should be attributes grouped by id, like core User or extensions
func getAttributeGroupSchema(groupT reflect.Type) (attributesSchema []AttributeSchema, err error) {
	subAttrCharacs, err := scimtag.GetSCIMCharacsInSubAttributes(groupT)
	if err != nil {
		return nil, err
	}
	for _, v := range subAttrCharacs {
		attrSchema, err := getAttributeSchema(v.Field.Type, v.Tag)
		if err != nil {
			return nil, err
		}
		attributesSchema = append(attributesSchema, attrSchema)
	}
	return
}

func getAttributeSchema(fieldT reflect.Type, scimCharacs scimtag.Characteristics) (AttributeSchema, error) {
	var attrSchema AttributeSchema
	attrSchema.Name = scimCharacs.Name
	tKind := fieldT.Kind()
	if tKind == reflect.Array || tKind == reflect.Slice {
		attrSchema.MultiValued = true
		fieldT = fieldT.Elem()
	}

	var err error
	attrSchema.Type, err = GetSCIMDataType(fieldT, scimCharacs.Name)
	if err != nil {
		return AttributeSchema{}, err
	}
	if attrSchema.Type == "complex" {
		attrSchema.SubAttributes, err = getAttributeGroupSchema(fieldT)
		if err != nil {
			return AttributeSchema{}, err
		}
	}
	// common in schema
	attrSchema.Required = scimCharacs.Required
	attrSchema.Mutability = scimCharacs.Mutability
	attrSchema.Returned = scimCharacs.Returned
	attrSchema.Uniqueness = util.ToPtr(scimCharacs.Uniqueness.String())
	if attrSchema.Type == "boolean" {
		attrSchema.Uniqueness = nil
	}

	if attrSchema.Type == "string" || attrSchema.Type == "reference" {
		attrSchema.CaseExact = &scimCharacs.CaseExact
	}

	attrSchema.ReferenceTypes = scimCharacs.ReferenceTypes // will omit when empty
	if attrSchema.Type != "complex" {
		// azure support, empty canonical value returned in schema
		if len(scimCharacs.CanonicalValues) != 0 {
			if len(scimCharacs.CanonicalValues) == 2 && scimCharacs.CanonicalValues[0] == "" && scimCharacs.CanonicalValues[1] == "" {
				attrSchema.CanonicalValues = util.ToPtr([]string{})
			} else {
				attrSchema.CanonicalValues = util.ToPtr(scimCharacs.CanonicalValues)
			}
		}
	}

	return attrSchema, nil
}

func GetSCIMDataType(t reflect.Type, scimAttrName string) (string, error) {
	kind := t.Kind()
	switch kind {
	case reflect.Bool:
		return "boolean", nil
	case reflect.Int:
		return "integer", nil
	case reflect.Float64, reflect.Float32:
		return "decimal", nil
	case reflect.String:
		if scimAttrName == "$ref" {
			return "reference", nil
		} else {
			return "string", nil
		}
	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			return "dateTime", nil
		} else {
			return "complex", nil
		}
	default:
		return "", errors.Errorf("failed to get SCIM attribute schema (type:%s, kind:%s)", t, kind)
	}
}
