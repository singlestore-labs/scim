package scimprotocol

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/memsql/errors"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/scimtag"
)

// EvalHelper is a helper function that evaluate filter expression against to the Object Reflect Value
// scimCharacs should be nil when object is not SCIM attribute
//
// azureFilterAdd is hacky way to compatible with azure's 'add' patch.
// Following patch should add new email that fit the filter and value.
// since it's hacky way so currently only support following two, no more complex op.
//
//	{
//		"op": "add",
//		"path": "emails[type eq \"work\"].value",
//		"value": "zoey@mcglynn.name"
//	},
//	{
//		"op": "add",
//		"path": "emails[type eq \"work\"].primary",
//		"value": true
//	},
func EvalHelper(e Expression, p *Node, objV reflect.Value, scimCharacs *scimtag.Characteristics, azureFilterAdd bool) (bool, error) {
	if p == nil {
		return false, errors.Errorf("unexpected error, path linked node should at least have a dummy head")
	}

	t := objV.Type()
	if t.Kind() == reflect.Ptr {
		objV = objV.Elem()
		t = objV.Type()
	}
	if azureFilterAdd && !objV.CanSet() {
		return false, errors.Errorf("with azure filter add option, input object does not support value setting")
	}

	// first call input path node should be dummyHead
	p = p.Next
	// if last one, do compare with scim characts
	if p == nil {
		return CompareValueAddIfAzure(objV, scimCharacs, e.CompareOp, e.Value, azureFilterAdd)
	}
	// else, continue walk down
	switch loc := p.Value.(type) {
	case string:
		// filter over array eg: emails.type eq "work"
		switch t.Kind() {
		case reflect.Array, reflect.Slice:
			for i := 0; i < objV.Len(); i++ {
				v := objV.Index(i)
				foundField, characs, err := scimtag.GetSCIMCharacs(v.Type(), loc)
				if err != nil {
					if errors.Is(err, scimerror.ErrNotFound) {
						return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "filter target not found %s (path:%v, objectType:%s)", loc, e.Path, v.Type()))
					}
					return false, err
				}
				targetValue := v.FieldByIndex(foundField.Index)
				if p.Next != nil && p.Next.Value != nil {
					// ref: https://datatracker.ietf.org/doc/html/rfc7643#section-1.2
					return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("invalid filter target on %s, simple attribute without sub-attribute, rfc7643#section-1.2", loc))
				}
				result, err := EvalHelper(e, p, targetValue, &characs, false) // should not support 'azure filter add' here
				if err != nil || result {
					return result, err
				}
			}
			// not found
			return false, nil
		default:
			foundField, characs, err := scimtag.GetSCIMCharacs(objV.Type(), loc)
			if err != nil {
				return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "not found target %s (path:%v, obj:%s)", loc, e.Path, objV.Type()))
			}
			targetValue := objV.FieldByIndex(foundField.Index)
			// if still have path, go next
			return EvalHelper(e, p, targetValue, &characs, azureFilterAdd)
		}
	case OrExpression:
		// when input object is list of complex struct like 'emails'
		switch t.Kind() {
		case reflect.Array, reflect.Slice:
			for i := 0; i < objV.Len(); i++ {
				v := objV.Index(i)
				pass, err := loc.Eval(v, false)
				if err != nil {
					return false, err
				}
				if pass {
					result, err := EvalHelper(e, p, v, scimCharacs, azureFilterAdd)
					if err != nil || result {
						return result, err
					}
				}
			}
			return false, nil // not found
		default:
			return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("filter does not support sub-filter on non-multi-value attributes (%s, kind:%+v)", t, t.Kind()))
		}
	default:
		return false, errors.Errorf("unsupported path node type %T", loc)
	}
}

func CompareValueAddIfAzure(target reflect.Value, targetCharacs *scimtag.Characteristics, op string, value string, azureAdd bool) (bool, error) {
	if op == "" && value == "" {
		return true, nil // no filter
	}
	if targetCharacs == nil {
		return false, errors.Errorf("unexpected error, compare on non-SCIM-attribute, %s", target.Type())
	}

	t := target.Type()
	switch t.Kind() {
	case reflect.Array, reflect.Slice:
		// true if any element matches
		for i := 0; i < target.Len(); i++ {
			v := target.Index(i)
			foundField, characs, err := scimtag.GetSCIMCharacs(v.Type(), "value") // default check 'value' for filter like `emails co "example.com"`
			if err != nil {
				return false, errors.Wrapf(err, "failed compare with multi-value attribute %s", targetCharacs.Name)
			}
			targetValue := v.FieldByIndex(foundField.Index)
			result, err := CompareValueAddIfAzure(targetValue, &characs, op, value, false)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	case reflect.String:
		targetValue := target.String()
		if !(len(value) > 0 && value[0] == '"' && value[len(value)-1] == '"') {
			return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("cannot compare string with non-string on %s with %s", targetCharacs.Name, value))
		}
		if azureAdd && op == "eq" {
			if !target.CanSet() {
				return false, errors.Errorf("failed to do azure add patch, cannot set %s (reflect value %s)", targetCharacs.Name, target)
			}
			target.Set(reflect.ValueOf(value[1 : len(value)-1]))
			return true, nil
		}
		return compareString(targetValue, op, value[1:len(value)-1], targetCharacs.CaseExact)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("cannot compare boolean on %s with %s", targetCharacs.Name, value))
		}
		if azureAdd && op == "eq" {
			if !target.CanSet() {
				return false, errors.Errorf("failed to do azure add patch, cannot set %s (reflect value %s)", targetCharacs.Name, target)
			}
			target.Set(reflect.ValueOf(boolValue))
			return true, nil
		}
		return compareBool(target.Bool(), op, false)
	case reflect.Struct:
		// for support time.Time
		if target.Type() == reflect.TypeOf(time.Time{}) {
			var valueTime time.Time
			if len(value) > 0 {
				err := json.Unmarshal([]byte(value), &valueTime)
				if err != nil {
					return false, errors.Wrapf(err, "failed compare 'time' %s", value)
				}
			} else {
				return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidValue, errors.Errorf("empty input timestamp"))
			}
			targetValue := target.Interface().(time.Time)
			return compareDateTime(targetValue, op, valueTime)
		} else {
			return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("could not compare on complex attribute on %s (type:%s)", targetCharacs.Name, target.Type()))
		}
	default:
		return false, errors.Errorf("unsupported compare type on %s (%s)", targetCharacs.Name, t.Kind())
	}
}

func compareDateTime(left time.Time, op string, right time.Time) (bool, error) {
	switch strings.ToLower(op) {
	case "pr":
		return left.Equal(time.Time{}), nil
	case "eq":
		return left.Equal(right), nil
	case "ne":
		return !left.Equal(right), nil
	case "gt":
		return left.After(right), nil
	case "ge":
		return !left.Before(right), nil
	case "lt":
		return left.Before(right), nil
	case "le":
		return !left.After(right), nil
	default:
		return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("not support compare operation %s on %T", op, left))
	}
}

func compareString(left string, op string, right string, caseExact bool) (bool, error) {
	if !caseExact {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}
	switch strings.ToLower(op) {
	case "pr":
		return left != "", nil
	case "eq":
		return left == right, nil
	case "ne":
		return left != right, nil
	case "co":
		return strings.Contains(left, right), nil
	case "sw":
		return strings.HasPrefix(left, right), nil
	case "ew":
		return strings.HasSuffix(left, right), nil
	case "gt":
		return left > right, nil
	case "ge":
		return left >= right, nil
	case "lt":
		return left < right, nil
	case "le":
		return left <= right, nil
	default:
		return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("not support compare operation %s on %T", op, left))
	}
}

func compareBool(left bool, op string, right bool) (bool, error) {
	switch strings.ToLower(op) {
	case "pr":
		return true, nil
	case "eq":
		return left == right, nil
	case "ne":
		return left != right, nil
	default:
		return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("not support compare operation %s on %T", op, left))
	}
}

func schemaFilterHelper(coreSchema string, extensions []SchemaExtention, inputValue string) (bool, error) {
	if coreSchema == "" {
		return false, nil
	}
	if !(len(inputValue) > 0 && inputValue[0] == '"' && inputValue[len(inputValue)-1] == '"') {
		return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("cannot compare string with non-string, %s", inputValue))
	}
	input := inputValue[1 : len(inputValue)-1]
	if input == coreSchema {
		return true, nil
	}
	for _, extension := range extensions {
		if extension.Schema == input {
			return true, nil
		}
	}
	return false, nil
}

func GetFilteredResources[T Resource](filter *OrExpression, inputs []T) ([]T, error) {
	checkFilter := func(resource T) (bool, error) {
		if filter != nil {
			return filter.Eval(reflect.ValueOf(resource), false)
		}
		return true, nil
	}

	filtered := []T{}
	for _, u := range inputs {
		pass, err := checkFilter(u)
		if err != nil {
			return nil, err
		}
		if pass {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}
