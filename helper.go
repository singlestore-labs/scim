package scimprotocol

import (
	"reflect"
	"strings"

	"github.com/memsql/errors"

	"singlestore.com/helios/scim/scimprotocol/scimerror"
	"singlestore.com/helios/scim/scimprotocol/scimtag"
)

// GetSchemaURIFromResource is a helper function to get schema URI and extension URIs for ResourceType from self-defined SCIM resource struct, like User, Group
// resourceV is only needed when you need filter out empty URI struct, like extension URI should not include in when it's empty during marshal
// if input type is not Resource, then return ErrNotFound error
// if input type is Resource but doesn't have URI, return error
func GetSchemaURIFromResource(resourceT reflect.Type, resourceVforFilterEmpty *reflect.Value) (coreSchemaURI string, extensions []SchemaExtention, err error) {
	if resourceT.Kind() != reflect.Struct {
		return "", nil, errors.Errorf("could not find schema from non-struct type (type:%s, kind:%s)", resourceT, resourceT.Kind())
	}
	if !resourceT.Implements(reflect.TypeOf((*Resource)(nil)).Elem()) {
		return "", nil, errors.Wrapf(scimerror.ErrNotFound, "input %s does not implement Resource", resourceT)
	}

	attrMap, err := scimtag.GetSCIMCharacsInSubAttributes(resourceT)
	if err != nil {
		return "", nil, err
	}
	for _, value := range attrMap {
		fieldCharacs := value.Tag
		// first one must be core schema
		if URIPattenRegexp.MatchString(fieldCharacs.Name) {
			// if URI field is empty, then not marshal it.
			if resourceVforFilterEmpty == nil || !resourceVforFilterEmpty.FieldByIndex(value.Field.Index).IsZero() {
				if strings.HasPrefix(fieldCharacs.Name, "urn:ietf:params:scim:schemas:core") {
					if coreSchemaURI == "" {
						coreSchemaURI = fieldCharacs.Name
					} else {
						return "", nil, errors.Errorf("multiple core schemas (%s, %s) in resource %s", coreSchemaURI, fieldCharacs.Name, resourceT)
					}
				} else {
					extensions = append(extensions, SchemaExtention{
						Schema:   fieldCharacs.Name,
						Required: fieldCharacs.Required,
					})
				}
			}
		}
	}
	if coreSchemaURI == "" {
		return "", nil, errors.Errorf("could not found core schema in resource %s", resourceT)
	}
	return
}
