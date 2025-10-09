package scimprotocol

import (
	"reflect"
	"testing"

	"github.com/singlestore-labs/scim/scimtag"
	"github.com/stretchr/testify/require"
)

func init() {
	scimtag.BuildAllSCIMCharacsCache(email{})
	scimtag.BuildAllSCIMCharacsCache(customizedEqualEmail{})
}

type email struct {
	Value   string `scim:"value"`
	Type    string `scim:"type,canonicalValues=work"` // azure only allow work // string array in tag separate by space
	Primary bool   `scim:"primary,returned=always"`
}

func TestAppendMultivalueAttr(t *testing.T) {
	t.Parallel()

	e1 := []email{
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
		{
			Value:   "v2",
			Type:    "t2",
			Primary: true,
		},
	}

	e2 := []email{
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
		{
			Value:   "v3",
			Type:    "t3",
			Primary: true,
		},
	}

	result, err := appendMultiValueAttr(reflect.ValueOf(e1), reflect.ValueOf(e2))
	require.NoError(t, err)
	t.Log(result.Interface())
	require.Equal(t, []email{
		{
			Value:   "v2",
			Type:    "t2",
			Primary: false,
		},
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
		{
			Value:   "v3",
			Type:    "t3",
			Primary: true,
		},
	}, result.Interface().([]email))
}

func TestUnsetPrimary(t *testing.T) {
	t.Parallel()

	e1 := []email{
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
		{
			Value:   "v2",
			Type:    "t2",
			Primary: true,
		},
	}

	t.Log("test unset primary")
	e1V := reflect.ValueOf(e1)
	err := unSetPrimary(e1V.Index(1))
	require.NoError(t, err)
	t.Log(e1V.Interface())
}

type customizedEqualEmail struct {
	Value   string `scim:"value"`
	Type    string `scim:"type,canonicalValues=work"` // azure only allow work // string array in tag separate by space
	Primary bool   `scim:"primary,returned=always"`
}

var _ MultiValueElement = (*customizedEqualEmail)(nil)

func (c customizedEqualEmail) isEqual(input MultiValueElement) bool {
	email, ok := input.(customizedEqualEmail)
	if !ok {
		return false
	}
	return c.Value == email.Value
}

func TestAppendMultivalueAttrWithCustomizedEqual(t *testing.T) {
	t.Parallel()

	e1 := []customizedEqualEmail{
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
		{
			Value:   "v2",
			Type:    "t2",
			Primary: true,
		},
	}

	e2 := []customizedEqualEmail{
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
		{
			Value:   "v2", // same value
			Type:    "t3",
			Primary: true,
		},
	}

	result, err := appendMultiValueAttr(reflect.ValueOf(e1), reflect.ValueOf(e2))
	require.NoError(t, err)
	t.Log(result.Interface())
	require.Equal(t, []customizedEqualEmail{
		{
			Value:   "v2",
			Type:    "t2",
			Primary: true,
		},
		{
			Value:   "v1",
			Type:    "t1",
			Primary: false,
		},
	}, result.Interface().([]customizedEqualEmail))
}
