package scimprotocol

import (
	"encoding/json"
	"reflect"

	"github.com/memsql/errors"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/scimtag"
)

type PatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

func UnmarshalPatchRequest(data []byte) ([]PatchOperation, error) {
	patchReqeust := struct {
		Schemas    []string         `json:"schemas"`
		Operations []PatchOperation `json:"Operations"`
	}{}
	err := json.Unmarshal(data, &patchReqeust)
	if err != nil {
		return nil, err
	}
	if len(patchReqeust.Schemas) != 1 || patchReqeust.Schemas[0] != "urn:ietf:params:scim:api:messages:2.0:PatchOp" {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidSyntax, errors.Errorf("invalid patch schema, %v", patchReqeust.Schemas))
	}
	return patchReqeust.Operations, nil
}

// path = nil:		patch on the end of path
// path = &Node:	patch on root
func Patch(objV reflect.Value, path *Node, op string, value []byte) error {
	t := objV.Type()
	if !objV.CanSet() {
		return errors.Errorf("patch object cannot be set")
	}
	if t.Kind() == reflect.Ptr {
		objV = objV.Elem()
		if !objV.IsValid() {
			return errors.Errorf("input patch object is not valid (t:%s, v:%s)", t, objV)
		}
		t = objV.Type()
	}

	isDummyhead := path != nil && path.Value == nil
	if isDummyhead && path.Next == nil {
		// no path, patch at root
		return patchOnRoot(t, objV, op, value)
	}

	path = path.Next

	if path == nil {
		if _, ok := objV.Interface().(ResourceID); ok {
			return patchOnResourceID(t, objV, op, value)
		}
		// goes to the end, patch separate
		switch t.Kind() {
		case reflect.Struct:
			return patchOnStruct(t, objV, op, value)
		case reflect.Array, reflect.Slice:
			return patchOnSlice(t, objV, op, value)
		case reflect.String:
			return patchOnString(objV, op, value)
		case reflect.Bool:
			return patchOnBool(objV, op, value)
		}
		return nil
	}

	switch p := path.Value.(type) {
	case string:
		// attribute must struct to be able to walk down
		if t.Kind() != reflect.Struct {
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidPath, errors.Errorf("cannot patch simple attribute %s (object type:%s, kind:%s)", p, t, t.Kind()))
		}
		foundField, characs, err := scimtag.GetSCIMCharacs(t, p)
		if err != nil {
			if errors.Is(err, scimerror.ErrNotFound) {
				err = scimerror.NewBadRequestSCIMErr(scimerror.InvalidPath, errors.Wrapf(err, "patch target %s (obj:%s) not found", p, objV.Type()))
			}
			return err
		}
		if characs.Mutability == scimtag.Immutable || characs.Mutability == scimtag.ReadOnly {
			return scimerror.NewBadRequestSCIMErr(scimerror.MutabilityError, errors.Errorf("cannot patch '%s' attribute with mutability %s", characs.Mutability, p))
		}
		targetValue := objV.FieldByIndex(foundField.Index)
		err = Patch(targetValue, path, op, value)
		if err != nil {
			return err
		}
		// after change, do the 'required' check
		if characs.Required {
			if targetValue.IsZero() {
				return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("cannot be empty value, %s is required", p))
			}
		}
	case OrExpression:
		// go over all element find those passed filter and patch
		// object should be array of struct, like 'emails'
		if t.Kind() != reflect.Array && t.Kind() != reflect.Slice {
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("single-value attribute does not support filtering for patch (attribute type:%s, kind:%s)", t, t.Kind()))
		}
		newSlice := reflect.New(t).Elem()
		found := false
		for i := 0; i < objV.Len(); i++ {
			v := objV.Index(i)
			// check filter
			pass, err := p.Eval(v, false)
			if err != nil {
				return errors.Wrapf(err, "failed to find on patch object (filter:%s, obj:%s)", p.String(), v.Type())
			}
			if pass {
				found = true
				err = Patch(v, path, op, value)
				if err != nil {
					return errors.Wrapf(err, "failed to patch value (%s) on target object (%s)", string(value), v.Type())
				}
				if v.IsZero() {
					continue
				}
			}
			newSlice = reflect.Append(newSlice, v)
		}
		// for support azure filter add
		if !found && op == "add" {
			newElem := reflect.New(t.Elem()).Elem()
			pass, err := p.Eval(newElem, true)
			if err != nil {
				return errors.Wrapf(err, "failed to get new element from patch filter for azure path add (filter: %s, obj: %s)", p.String(), newElem.Type())
			}
			if !pass {
				return errors.Wrapf(err, "failed to do azure patch add")
			}
			err = Patch(newElem, path, op, value)
			if err != nil {
				return err
			}
			newSlice = reflect.Append(newSlice, newElem)
		}
		objV.Set(newSlice)
	}
	return nil
}

func patchOnRoot(objT reflect.Type, objV reflect.Value, patchOp string, patchValue []byte) error {
	if objT.Kind() != reflect.Struct {
		return errors.Errorf("patch on root must be struct but input is %s, %v", objT, objT.Kind())
	}
	switch patchOp {
	case "add", "replace":
		coreSchema, _, err := GetSchemaURIFromResource(objT, nil)
		if err != nil {
			return err
		}
		// go over each attribute and patch
		inputAttrMap := map[string]json.RawMessage{}
		err = json.Unmarshal(patchValue, &inputAttrMap)
		if err != nil {
			return errors.Wrapf(err, "could not json unmarshal (data:%s)", string(patchValue))
		}
		for name, v := range inputAttrMap {
			// compatible for azure, eg:
			// {
			// 	"op": "replace",
			// 	"value": {
			// 		"name.familyName": "Unua",
			// 		"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:employeeNumber": "Aklq"
			// 	}
			// }
			path, err := ParsePath(name)
			if err != nil {
				return scimerror.NewBadRequestSCIMErr(scimerror.InvalidPath, errors.Wrapf(err, "failed to parse path %s", name))
			}
			err = Patch(objV, path.GetNodes(coreSchema), patchOp, v)
			if err != nil {
				return err
			}
		}
		return nil
	case "remove":
		return scimerror.NewBadRequestSCIMErr(scimerror.InvalidPath, errors.Errorf("remove operation on patch require path"))
	default:
		return scimerror.NewBadRequestSCIMErr(scimerror.InvalidSyntax, errors.Errorf("unsupported patch operation %s", patchOp))
	}
}

// patchOnStruct patch on struct type, in SCIM it should only be complex attribute like email, role ...
func patchOnStruct(objT reflect.Type, objV reflect.Value, patchOp string, patchValue []byte) error {
	patchValueStr := string(patchValue)
	isValueString := len(patchValueStr) > 1 && patchValueStr[0] == '"' && patchValueStr[len(patchValueStr)-1] == '"'
	if isValueString {
		// when not specify field, we only allow patch on 'value' field on default for string value
		sf, characs, err := scimtag.GetSCIMCharacs(objT, "value")
		if errors.Is(err, scimerror.ErrNotFound) {
			return errors.Errorf("could not patch complex attribute (%s) with string %s", objT, string(patchValue))
		}
		if err != nil {
			return err
		}
		if characs.Mutability == scimtag.Immutable || characs.Mutability == scimtag.ReadOnly {
			return scimerror.NewBadRequestSCIMErr(scimerror.MutabilityError, errors.Errorf("cannot patch %s 'value' attribute", characs.Mutability))
		}
		fieldV := objV.FieldByIndex(sf.Index)
		fieldV.Set(reflect.ValueOf(patchValueStr[1 : len(patchValueStr)-1]))
		return nil
	}

	switch patchOp {
	case "add":
		updatedObj := objV.Addr().Interface()
		if err := Unmarshal(patchValue, updatedObj); err != nil {
			return errors.Wrapf(err, "could not unmarshal patch value (type:%s,  data:%s)", objT, patchValue)
		}
		if !objV.CanSet() {
			return errors.Errorf("input object cannot be set (type:%s)", objT)
		}
		objV.Set(reflect.ValueOf(updatedObj).Elem())
	case "replace":
		newObj := reflect.New(objT).Interface()
		if err := Unmarshal(patchValue, newObj); err != nil {
			return errors.Wrapf(err, "could not unmarshal patch value (type:%s, data:%s)", objT, patchValue)
		}
		objV.Set(reflect.ValueOf(newObj).Elem())
	case "remove":
		objV.Set(reflect.Zero(objT))
	default:
		return errors.Errorf("unsupported patch operation %s", patchOp)
	}
	return nil
}

func patchOnResourceID(objT reflect.Type, objV reflect.Value, patchOp string, patchValue []byte) error {
	if _, ok := objV.Interface().(ResourceID); ok {
		switch patchOp {
		case "add", "replace":
			newObj := reflect.New(objT).Interface()
			if err := Unmarshal(patchValue, newObj); err != nil {
				return errors.Wrapf(err, "could not unmarshal patch value (type:%s, data:%s)", objT.Name(), patchValue)
			}
			objV.Set(reflect.ValueOf(newObj).Elem())
		case "remove":
			objV.Set(reflect.Zero(objT))
		default:
			return errors.Errorf("unsupported patch operation %s", patchOp)
		}
	} else {
		return errors.Errorf("cannot patch interface type %s", objT.Name())
	}
	return nil
}

func patchOnSlice(objT reflect.Type, objV reflect.Value, patchOp string, patchValue []byte) error {
	// patch on list
	switch patchOp {
	case "add":
		inputArray := reflect.New(objT).Interface()
		if err := Unmarshal(patchValue, inputArray); err != nil {
			return errors.Wrapf(err, "could not unmarshal patch value (type:%s, data:%s)", objT, patchValue)
		}
		inputV := reflect.ValueOf(inputArray).Elem()
		newSlice, err := appendMultiValueAttr(objV, inputV)
		if err != nil {
			return err
		}
		objV.Set(newSlice)
	case "replace":
		inputArray := reflect.New(objT).Interface()
		if err := Unmarshal(patchValue, inputArray); err != nil {
			return errors.Wrapf(err, "could not unmarshal patch value (type:%s, data:%s)", objT, patchValue)
		}
		objV.Set(reflect.ValueOf(inputArray).Elem())
	case "remove":
		objV.Set(reflect.Zero(objT))
	default:
		return errors.Errorf("unsupported patch operation %s", patchOp)
	}
	return nil
}

func patchOnString(objV reflect.Value, patchOp string, patchValue []byte) error {
	switch patchOp {
	case "add", "replace":
		var stringValue string
		err := json.Unmarshal(patchValue, &stringValue)
		if err != nil {
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("cannot patch string with non-string, %s: %w", string(patchValue), err))
		}
		objV.Set(reflect.ValueOf(stringValue))
	case "remove":
		objV.Set(reflect.ValueOf(""))
	default:
		return errors.Errorf("unsupported patch operation %s", patchOp)
	}
	return nil
}

func patchOnBool(objV reflect.Value, patchOp string, patchValue []byte) error {
	switch patchOp {
	case "add", "replace":
		switch string(patchValue) {
		case "true":
			objV.Set(reflect.ValueOf(true))
		case "false":
			objV.Set(reflect.ValueOf(false))
		default:
			return scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("cannot patch bool with non-bool value, %s", string(patchValue)))
		}
	case "remove":
		objV.Set(reflect.ValueOf(false))
	default:
		return errors.Errorf("unsupported patch operation %s", patchOp)
	}
	return nil
}

// movePrimaryFirst, modify exist reflect.Value of slice to let primary element at first, index 0.
// return error if there is more than one element is primary
func movePrimaryFirst(input reflect.Value) error {
	if input.Type().Kind() != reflect.Array && input.Type().Kind() != reflect.Slice {
		return errors.Errorf("cannot move primary on a non-slice attribute")
	}
	found := false
	for i := 0; i < input.Len(); i++ {
		primary, err := isPrimary(input.Index(i))
		if err != nil {
			if errors.Is(err, scimerror.ErrNotFound) {
				return nil // skip move primary, no such field
			}
			return err
		}
		if primary && i == 0 {
			return nil
		}
		if primary {
			if found {
				return errors.Errorf("input multi-value attribute with more than one primary element")
			}
			cur := input.Index(i)
			first := input.Index(0)
			temp := reflect.New(first.Type()).Elem()
			temp.Set(first)

			first.Set(cur)
			cur.Set(temp)
			found = true
		}
	}
	return nil
}

// appendMultiValueAttr append reflect value of input slice to the existing reflect value slice.
// the output will should be 1. only one primary 2. no duplicate
func appendMultiValueAttr(s reflect.Value, t reflect.Value) (reflect.Value, error) {
	// if same element then don't add
	newElements := reflect.New(t.Type()).Elem()
	for i := 0; i < t.Len(); i++ {
		v := t.Index(i)
		if found := findInSlice(s, v); !found {
			newElements = reflect.Append(newElements, v)
		}
	}

	// sort primary
	err := movePrimaryFirst(s)
	if err != nil {
		return reflect.Value{}, err
	}
	err = movePrimaryFirst(newElements)
	if err != nil {
		return reflect.Value{}, err
	}

	// handle primary conflict
	if newElements.Len() > 0 {
		if primary, err := isPrimary(newElements.Index(0)); err == nil && primary {
			// unset primary for exist one
			if s.Len() > 0 {
				err := unSetPrimary(s.Index(0))
				if err != nil {
					return reflect.Value{}, err
				}
			}
		} else if err != nil && !errors.Is(err, scimerror.ErrNotFound) {
			return reflect.Value{}, err
		}
		// else, ignore for ErrNotFound
	}
	return reflect.AppendSlice(s, newElements), nil
}

// findInSlice find element in slice.
// implement interface `MultiValueElement` with isEqual() for customized compare.
// default will check if whole object is equal.
func findInSlice(slice reflect.Value, element reflect.Value) bool {
	for i := 0; i < slice.Len(); i++ {
		v := slice.Index(i)
		vInf := v.Interface()
		eInf := element.Interface()
		if elemV, ok := vInf.(MultiValueElement); ok {
			if elemIn, ok := eInf.(MultiValueElement); ok {
				if elemV.isEqual(elemIn) {
					return true
				}
			}
		} else if v.Interface() == element.Interface() { // default
			return true
		}
	}
	return false
}

func unSetPrimary(v reflect.Value) error {
	foundField, _, err := scimtag.GetSCIMCharacs(v.Type(), "primary")
	if err != nil && !errors.Is(err, scimerror.ErrNotFound) { // skip if no primary field
		return errors.Wrapf(err, "failed to unset 'primary' field (%s)", v.Type())
	}
	targetValue := v.FieldByIndex(foundField.Index)
	if !targetValue.CanSet() {
		return errors.Errorf("could not change 'primary' filed (%s)", v.Type())
	}
	targetValue.Set(reflect.ValueOf(false))
	return nil
}

func isPrimary(v reflect.Value) (bool, error) {
	foundField, _, err := scimtag.GetSCIMCharacs(v.Type(), "primary")
	if err != nil {
		return false, errors.Wrapf(err, "failed to get 'primary' field (%s)", v.Type())
	}
	targetValue := v.FieldByIndex(foundField.Index)
	return targetValue.Bool(), nil
}
