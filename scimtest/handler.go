package scimtest

import (
	"net/http"

	"github.com/muir/nchi"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/util"
)

const DefaultPrefix = "/scim/v2"

// NewMux returns an HTTP handler that serves the in-memory example SCIM
// server under prefix (for example /scim/v2).
func NewMux(trace util.Trace, storage *Storage, prefix string) http.Handler {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	r := nchi.NewRouter()
	scimServer := NewServer(trace, storage)
	r.Route(prefix, scimprotocol.SCIMRouter(tracerOrDiscard(trace), scimServer))
	return r
}

func tracerOrDiscard(trace util.Trace) util.Trace {
	if trace == nil {
		return discardTrace{}
	}
	return trace
}

type discardTrace struct{}

func (discardTrace) Logf(string, ...interface{}) {}
