package scimtag

import (
	"reflect"

	"github.com/memsql/errors"
	"github.com/muir/reflectutils"
	"github.com/singlestore-labs/scim/scimerror"
)

// map[Struct Type]map[AttributeName]CacheValue
var cache = make(map[reflect.Type]map[string]CacheValue)

type CacheValue struct {
	Field reflect.StructField
	Tag   Characteristics
}

func BuildAllSCIMCharacsCache(obj ...any) {
	for _, o := range obj {
		if err := buildSCIMCharacsCache(reflect.TypeOf(o)); err != nil {
			panic(err)
		}
	}
}

// buildSCIMCharacsCache Must called with SCIM resourceType like User, Group.
// it must called before any SCIM action, like in Init()
func buildSCIMCharacsCache(t reflect.Type) error {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	err := reflectutils.WalkStructElementsWithError(t, func(f reflect.StructField) error {
		var scimCharacs Characteristics
		tagSet := reflectutils.SplitTag(f.Tag).Set()
		tag, ok := tagSet.Lookup(TagName)
		if !ok {
			return reflectutils.DoNotRecurseSignalErr // skip if no scim tag
		}
		err := tag.Fill(&scimCharacs)
		if err != nil {
			return errors.Wrapf(err, "failed to get tag %s from struct field (%+v)", TagName, f)
		}
		if cache[t] == nil {
			cache[t] = make(map[string]CacheValue)
		}
		cache[t][scimCharacs.Name] = CacheValue{Field: f, Tag: scimCharacs}

		// recursive
		err = buildSCIMCharacsCache(f.Type)
		if err != nil {
			return nil
		}
		return reflectutils.DoNotRecurseSignalErr
	})
	if errors.Is(err, reflectutils.DoNotRecurseSignalErr) {
		return nil
	}
	return err
}

func GetSCIMCharacsInSubAttributes(t reflect.Type) (map[string]CacheValue, error) {
	if t.Kind() != reflect.Struct {
		return nil, errors.Errorf("invalid path, looking up field in non-struct, %s, kind is %s", t, t.Kind())
	}
	if len(cache) == 0 {
		return nil, errors.Errorf("empty SCIM tag cache, please build scim tag cache first")
	}
	attrsMap, ok := cache[t]
	if !ok {
		return nil, errors.Wrapf(scimerror.ErrNotFound, "not found type '%s' in cache", t)
	}
	return attrsMap, nil
}

func GetSCIMCharacs(t reflect.Type, attributeName string) (reflect.StructField, Characteristics, error) {
	attrsMap, err := GetSCIMCharacsInSubAttributes(t)
	if err != nil {
		return reflect.StructField{}, Characteristics{}, err
	}
	value, ok := attrsMap[attributeName]
	if !ok {
		return reflect.StructField{}, Characteristics{}, errors.Wrapf(scimerror.ErrNotFound, "not found attribute '%s' in '%s' at SCIM tag cache", attributeName, t)
	}
	return value.Field, value.Tag, nil
}
