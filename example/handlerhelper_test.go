package example

import (
	"testing"

	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/stretchr/testify/require"
)

type tNum struct {
	scimprotocol.SCIMResourceMarker
	num int
}

func newTNum(num int) tNum {
	return tNum{
		num: num,
	}
}

func TestGetCurrentPageResources(t *testing.T) {
	{
		t.Log("test normal, startIndex 1, count 2, expect [1,2]")
		actual, err := getCurrentPageResources([]tNum{newTNum(1), newTNum(2), newTNum(3), newTNum(4), newTNum(5)}, 1, 2)
		require.NoError(t, err)
		require.Equal(t, []tNum{newTNum(1), newTNum(2)}, actual)
	}
	{
		t.Log("test count = 0, startIndex 1, count 0, expect []")
		actual, err := getCurrentPageResources([]tNum{newTNum(1), newTNum(2), newTNum(3), newTNum(4), newTNum(5)}, 1, 0)
		require.NoError(t, err)
		require.Equal(t, []tNum{}, actual)
	}
	{
		t.Log("test count = 1, startIndex 1, count 1, expect []")
		actual, err := getCurrentPageResources([]tNum{newTNum(1), newTNum(2), newTNum(3), newTNum(4), newTNum(5)}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, []tNum{newTNum(1)}, actual)
	}
	{
		t.Log("test start = end, startIndex 5, count 1, expect []")
		actual, err := getCurrentPageResources([]tNum{newTNum(1), newTNum(2), newTNum(3), newTNum(4), newTNum(5)}, 5, 1)
		require.NoError(t, err)
		require.Equal(t, []tNum{newTNum(5)}, actual)
	}
	{
		t.Log("test start > end, startIndex 6, count 1, expect []")
		actual, err := getCurrentPageResources([]tNum{newTNum(1), newTNum(2), newTNum(3), newTNum(4), newTNum(5)}, 6, 1)
		require.NoError(t, err)
		require.Equal(t, []tNum{}, actual)
	}
}
