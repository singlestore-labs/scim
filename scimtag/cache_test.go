package scimtag

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	BuildAllSCIMCharacsCache(T1{})
}

type T1 struct {
	N1  string
	T2  `scim:"t2"`
	F12 T3 `scim:"f12,ignoreUnmarshal,returned=always"`
}

type T2 struct {
	F21 bool   `scim:"f21,returned=keepEmpty"`
	F22 string `scim:"f22,caseExact"`
	F23 string `scim:"f23,required"`
	F24 []T4   `scim:"f24,required"`
	F25 T4     `scim:"f25"`
}

type T3 struct {
	F31 string `scim:"f31,returned=always"`
	F32 bool   `scim:"f32,returned=always"`
}
type T4 struct {
	F41 string `scim:"f41"`
	F42 string `scim:"f42"`
}

func TestSCIMTagBuild(t *testing.T) {
	t.Parallel()
	require.Contains(t, fmt.Sprint(cache), "T1")
	require.Contains(t, fmt.Sprint(cache), "T2")
	require.Contains(t, fmt.Sprint(cache), "T3")
	require.Contains(t, fmt.Sprint(cache), "T4")
	require.NotContains(t, fmt.Sprint(cache), "N1")

	_, scimCharacs, err := GetSCIMCharacs(reflect.TypeOf(T1{}), "t2")
	require.NoError(t, err)
	require.Equal(t, "t2", scimCharacs.Name)
	_, scimCharacs, err = GetSCIMCharacs(reflect.TypeOf(T2{}), "f24")
	require.NoError(t, err)
	require.Equal(t, "f24", scimCharacs.Name)
	require.True(t, scimCharacs.Required)
	_, _, err = GetSCIMCharacs(reflect.TypeOf(T1{}), "")
	require.Error(t, err)
}
