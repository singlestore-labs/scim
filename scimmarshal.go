package scimprotocol

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/memsql/errors"
	"github.com/muir/reflectutils"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/scimtag"
)

// SCIMMarshaler and SCIMUnmarshaler allow customized marshal and unmarshal for SCIM
// Note: if you choose to have SCIMMarshaler or SCIMUnmarshaler, means you take full control of marshal.
// And the patch and filter may not work as expected.
type SCIMMarshaler interface {
	MarshalSCIM() ([]byte, error)
}

type SCIMUnmarshaler interface {
	UnmarshalSCIM([]byte) error
}

type PrimaryDataType interface {
	SCIMCompareValue(op string, stringValue string, azureAdd bool) (bool, error)
}

func Marshal(obj any) ([]byte, error) {
	return MarshalWithSelectedAttr(obj, nil, false)
}

func MarshalWithSelectedAttr(obj any, selectedAttr []string, excludeSelected bool) (_ []byte, err error) {
	objV := reflect.ValueOf(obj)
	objT := objV.Type()
	if objV.Type().Kind() == reflect.Pointer {
		objV = objV.Elem()
		if !objV.IsValid() {
			return nil, errors.Errorf("invalid SCIM marshal input object")
		}
		objT = objV.Type()
	}

	if objT == reflect.TypeOf(json.RawMessage{}) {
		return obj.(json.RawMessage), nil
	}
	if objT == reflect.TypeOf([]byte{}) {
		return obj.([]byte), nil
	}

	resource, customizedMarshal := obj.(SCIMMarshaler)
	if customizedMarshal {
		return resource.MarshalSCIM()
	}

	if objT.Kind() == reflect.Array || objT.Kind() == reflect.Slice {
		objV := reflect.ValueOf(obj)
		jsonList := []json.RawMessage{}
		for i := 0; i < objV.Len(); i++ {
			j, err := MarshalWithSelectedAttr(objV.Index(i).Interface(), selectedAttr, excludeSelected)
			if err != nil {
				return nil, errors.Errorf("cannot marshal slice/array (%s, %+v): %w\n", objV.Type(), obj, err)
			}
			jsonList = append(jsonList, j)
		}
		result, err := json.Marshal(jsonList)
		return result, errors.WithStack(err)
	}

	if resource, ok := obj.(Resource); ok {
		return resourceMarshal(resource, selectedAttr, excludeSelected)
	}
	return nil, errors.Errorf("input object neither 'Resource' nor 'SCIMMarshaler', %T, %+v\n", obj, obj)
}

// resourceMarshal helps marshal resource according to the SCIM attribute characteristics
func resourceMarshal(obj Resource, selectAttr []string, excludeSelected bool) ([]byte, error) {
	objV := reflect.ValueOf(obj)

	coreSchema, _, err := GetSchemaURIFromResource(objV.Type(), nil)
	if err != nil {
		return nil, err
	}

	attributesPath := make([]*Node, len(selectAttr))
	for i, a := range selectAttr {
		p, err := ParsePath(a)
		if err != nil {
			return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidPath, errors.Wrapf(err, "failed to parse path %s", a))
		}
		if p.Filter != nil {
			return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidPath, errors.Wrapf(err, "could not select attribute with filter %s", a))
		}
		dummyhaed := p.GetNodes("")
		attributesPath[i] = dummyhaed
	}

	untypedField, err := marshalHelper(objV, coreSchema, attributesPath, excludeSelected)
	if err != nil {
		return nil, nil
	}

	mapField, ok := untypedField.(map[string]any)
	if !ok {
		return nil, errors.Errorf("failed convert to field map, %+v", untypedField)
	}

	schemaURIs := []string{coreSchema}
	for k := range mapField {
		if URIPattenRegexp.MatchString(k) {
			schemaURIs = append(schemaURIs, k)
		}
	}
	mapField["schemas"] = schemaURIs
	result, err := json.Marshal(mapField)
	return result, errors.WithStack(err)
}

// marshalHelper helps marshal recursively
func marshalHelper(objV reflect.Value, coreSchemaURI string, selectAttrs []*Node, exclude bool) (_ any, err error) {
	objT := objV.Type()
	defer func() {
		err = errors.Wrapf(err, "failed marshal %s (coreSchema:%s, exclude:%t, selectAttrs:%v)", objT, coreSchemaURI, exclude, selectAttrs)
	}()
	if !objV.CanInterface() {
		return nil, errors.Errorf("cannot interface reflect value %s", objV)
	}

	if isPrimarySCIMDataType(objV) {
		// when not belongs SCIM data type, do json marshal as default
		// rfc: https://datatracker.ietf.org/doc/html/rfc7643#section-2.3
		return objV.Interface(), nil
	}

	if objT.Kind() == reflect.Array || objT.Kind() == reflect.Slice {
		list := make([]any, objV.Len())
		for i := 0; i < objV.Len(); i++ {
			elem, err := marshalHelper(objV.Index(i), coreSchemaURI, selectAttrs, exclude)
			if err != nil {
				return nil, err
			}
			list[i] = elem
		}
		return list, nil
	}

	selectAttrs, err = movePathToNext(selectAttrs, nil)
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]any{}
	err = reflectutils.WalkStructElementsWithError(objT, func(sf reflect.StructField) error {
		var scimCharacs scimtag.Characteristics
		tagSet := reflectutils.SplitTag(sf.Tag).Set()
		scimTag, ok := tagSet.Lookup(scimtag.TagName)
		if !ok {
			return reflectutils.DoNotRecurseSignalErr // skip current
		}
		err = scimTag.Fill(&scimCharacs)
		if err != nil {
			return errors.Wrapf(err, "failed to get fill tag '%s' to struct field (%+v, obj:%s)", scimtag.TagName, sf, objT)
		}
		// read name and recursive down until primary
		if scimCharacs.Name == "" {
			return errors.Errorf("SCIM attribute name is required")
		}
		fieldV := objV.FieldByIndex(sf.Index)
		if scimCharacs.Name == coreSchemaURI {
			selectAttrs, err = movePathToNext(selectAttrs, &coreSchemaURI)
			if err != nil {
				return err
			}
			return nil // recursive to the struct
		}

		// returned=default will omit empty
		// returned=keepEmpty will keep empty
		// returned=request will must marshal when it's been requested
		// returned=never will never marshal the field even selected
		// returned=always will alway marshal the field even not selected
		nextPath := selectAttrs
		shouldMarshalField := func() bool {
			if scimCharacs.Returned == scimtag.Never {
				return false
			}
			if scimCharacs.Returned == scimtag.Always {
				return true
			}

			// check select attributes
			if len(selectAttrs) > 0 {
				foundPaths, isEndOfPath := findPathsForField(scimCharacs.Name, selectAttrs)
				found := len(foundPaths) > 0
				// if exclude  and found
				//						 and EndOfPath   then should remove attribute
				// 						 and !EndOfPath  then should keep looking down
				// if exclude  and !found  			     then should ignore and continue
				// if !exclude and found      			 then should keep looking down
				// if !exclude and !found 				 then should remove attribute
				if (found && exclude && isEndOfPath) || (!found && !exclude) {
					return false
				} else if (found && exclude && !isEndOfPath) || (!exclude && found) {
					// err = selectAttributeHelper(fieldV, foundPaths, exclude)
					// if err != nil {
					// 	return false
					// }
					nextPath = foundPaths
					return true
				}
			}

			// if no select, then rest check if requested then skip
			if scimCharacs.Returned == scimtag.Request {
				return false
			}

			// if default, check omitempty
			isEmptyArray := (fieldV.Type().Kind() == reflect.Array || fieldV.Type().Kind() == reflect.Slice) && fieldV.Len() == 0
			if scimCharacs.Returned != scimtag.KeepEmpty && (fieldV.IsZero() || isEmptyArray) {
				return false
			}
			return true
		}

		if shouldMarshalField() {
			var fields any
			fields, err = marshalHelper(fieldV, coreSchemaURI, nextPath, exclude)
			if err != nil || fields == nil {
				return err
			}
			fieldMap[scimCharacs.Name] = fields
		}
		return reflectutils.DoNotRecurseSignalErr
	})
	if err != nil {
		return nil, err
	}
	return fieldMap, nil
}

// Unmarshal not required to be resource, since we need it in patch to unmarshal attribute
func Unmarshal(data []byte, obj any) (err error) {
	defer func() {
		err = errors.Wrapf(err, "failed to SCIM unmarshal (%v)", obj)
	}()
	if customizedUnmarshal, ok := obj.(SCIMUnmarshaler); ok {
		return customizedUnmarshal.UnmarshalSCIM(data)
	}

	objT := reflect.TypeOf(obj)
	if objT.Kind() != reflect.Pointer {
		return errors.Errorf("SCIM unmarshal requires object pointer but input is %s, %+v", objT.Kind(), obj)
	}
	objV := reflect.ValueOf(obj).Elem()
	objT = objV.Type()
	if !objV.CanSet() {
		return errors.Errorf("failed to do scim unmarshal, input % cannot be set", objT)
	}
	if isPrimarySCIMDataType(objV) {
		// it's primary type
		return errors.WithStack(json.Unmarshal(data, obj))
	}

	if objT.Kind() == reflect.Array || objT.Kind() == reflect.Slice {
		jsonList := []json.RawMessage{}
		err := json.Unmarshal(data, &jsonList)
		if err != nil {
			return errors.WithStack(err)
		}
		newSliceV := reflect.New(objT).Elem()
		for _, j := range jsonList {
			newElemPtrV := reflect.New(objT.Elem())
			newElemPtr := newElemPtrV.Interface()
			err := Unmarshal(j, newElemPtr)
			if err != nil {
				return err
			}
			newSliceV = reflect.Append(newSliceV, newElemPtrV.Elem())
		}
		objV.Set(newSliceV)
		return nil
	}

	fieldMap := map[string]json.RawMessage{}
	err = json.Unmarshal(data, &fieldMap)
	if err != nil {
		return errors.WithStack(err)
	}
	// check schema
	var coreSchemaURI *string
	if _, ok := obj.(Resource); ok {
		schemasJSON, ok := fieldMap["schemas"]
		if !ok {
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("input json invalid, SCIM require 'schemas'"))
		}
		var schemas []string
		err := json.Unmarshal(schemasJSON, &schemas)
		if err != nil {
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Wrapf(err, "input json invalid, cannot unmarshal 'schemas'"))
		}
		coreURI, _, err := GetSchemaURIFromResource(objT, nil)
		if err != nil {
			return err
		}
		coreSchemaURI = &coreURI
		foundCore := false
		for _, s := range schemas {
			if coreURI == s {
				foundCore = true
			}
		}
		if !foundCore {
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("input schemas invalid, require %s", coreURI))
		}
	}

	// walk obj
	return reflectutils.WalkStructElementsWithError(objT, func(sf reflect.StructField) error {
		var scimCharacs scimtag.Characteristics
		tagSet := reflectutils.SplitTag(sf.Tag).Set()
		scimTag, ok := tagSet.Lookup(scimtag.TagName)
		if !ok {
			return reflectutils.DoNotRecurseSignalErr
		}
		err = scimTag.Fill(&scimCharacs)
		if err != nil {
			return errors.Wrapf(err, "failed to get tag '%s' from struct field (%+v, obj:%s)", scimtag.TagName, sf, objT)
		}
		if scimCharacs.IgnoreUnmarshal {
			return reflectutils.DoNotRecurseSignalErr
		}
		if scimCharacs.Name == "" {
			return errors.Errorf("SCIM attribute name is required")
		}
		if coreSchemaURI != nil && scimCharacs.Name == *coreSchemaURI {
			return nil // skip current field and recursive down
		}
		// if found in map then recursive down with reflect.value
		// set field with unmarshal value
		fieldJSON, ok := fieldMap[scimCharacs.Name]
		if !ok {
			if scimCharacs.Required {
				return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("attribute '%s' is required", scimCharacs.Name))
			}
			return reflectutils.DoNotRecurseSignalErr
		}
		fieldV := objV.FieldByIndex(sf.Index)
		fieldPtr := fieldV.Addr().Interface()
		err = Unmarshal(fieldJSON, fieldPtr)
		if err != nil {
			return err
		}
		return reflectutils.DoNotRecurseSignalErr
	})
}

// isPrimarySCIMDataType: check if it's Primary data type for SCIM,
// which means should not do SCIM unmarshal or keep recursive down for SCIM related operation
func isPrimarySCIMDataType(v reflect.Value) bool {
	if _, ok := v.Interface().(PrimaryDataType); ok {
		return true
	}
	t := v.Type()
	return (t.Kind() != reflect.Struct && t.Kind() != reflect.Array &&
		t.Kind() != reflect.Slice && t.Kind() != reflect.Map) ||
		t == reflect.TypeOf(time.Time{}) || t == reflect.TypeOf(uuid.UUID{})
}

// movePathToNext moves matched path to next node
// if input match is nil, move all the path to next
//
// case 1, move all to next:
//
//	before: paths[name, email->value] input: match = nil
//	after: paths[value] input: match = nil
//
// case 2, move some path with URI:
//
//	before: paths[urn:ietf:params:scim:schemas:core:2.0:User->name, email->value] input: match = urn:ietf:params:scim:schemas:core:2.0:User
//	after: paths[name, email->value] input: match = email
func movePathToNext(paths []*Node, match *string) ([]*Node, error) {
	var newPaths []*Node
	for _, p := range paths {
		if match == nil { // if nil, move all
			if p != nil && p.Next != nil && p.Next.Value != nil {
				newPaths = append(newPaths, p.Next)
			}
		} else {
			pStr, ok := p.Value.(string)
			if !ok {
				return nil, errors.Errorf("select attribute does not support filter (path:%+v)", p.Value)
			}
			if pStr == *match {
				if p != nil && p.Next != nil && p.Next.Value != nil {
					newPaths = append(newPaths, p.Next)
				}
			} else {
				newPaths = append(newPaths, p)
			}
		}
	}
	return newPaths, nil
}

func findPathsForField(cur string, paths []*Node) ([]*Node, bool) {
	var foundNodes []*Node
	isEndOfPath := false
	for _, p := range paths {
		if p != nil && p.Value == cur {
			if p.Next == nil || p.Next.Value == nil {
				isEndOfPath = true
			}
			foundNodes = append(foundNodes, p)
		}
	}
	return foundNodes, isEndOfPath
}
