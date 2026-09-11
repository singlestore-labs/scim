// Command scimtestd serves the in-memory example SCIM 2.0 server used by
// compatibility suites in GitHub Actions.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/singlestore-labs/scim/scimtest"
)

type logTrace struct{}

func (logTrace) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "listen address")
	prefix := flag.String("prefix", scimtest.DefaultPrefix, "SCIM base path")
	flag.Parse()

	storage := scimtest.NewInMemStorage()
	scimtest.SeedExampleData(storage)
	handler := scimtest.NewMux(logTrace{}, storage, *prefix)

	log.Printf("SCIM test server listening on http://%s%s (Bearer %s)", *addr, *prefix, scimtest.DummyToken)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
