package util

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func PrettyJSON(t *testing.T, data []byte) []byte {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, data, "", "\t")
	require.NoError(t, err, string(data))
	return prettyJSON.Bytes()
}
