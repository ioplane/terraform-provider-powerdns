//go:build acceptance

package testutil_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

// TestRecordFixtures captures real responses from the lab into testdata.
//
// It is a test rather than a script so it lives with the code it serves and
// runs under the same toolchain. Run it with:
//
//	task lab:up && task fixtures:record
//
// Recording is deliberately manual. A fixture that re-records itself on every
// run is not a fixture: the point is that a change in what PowerDNS sends shows
// up as a diff somebody reviews.
func TestRecordFixtures(t *testing.T) {
	if os.Getenv("RECORD_FIXTURES") == "" {
		t.Skip("set RECORD_FIXTURES=1 to re-record against the lab")
	}

	key := os.Getenv("PDNS_API_KEY")
	ctx := context.Background()

	type capture struct {
		product     testutil.Product
		base        string
		version     string
		name        string
		description string
		method      string
		path        string
		query       string
		body        any
	}

	auth := os.Getenv("PDNS_SERVER_URL")
	rec := os.Getenv("PDNS_RECURSOR_SERVER_URL")
	dd := os.Getenv("PDNS_DNSDIST_SERVER_URL")

	captures := []capture{
		{
			testutil.ProductAuth, auth, "5.1.3", "servers-list",
			"the server list, which is how the provider learns the API is reachable",
			http.MethodGet, "/api/v1/servers", "", nil,
		},
		{
			testutil.ProductAuth, auth, "5.1.3", "zone-create-native",
			"a Native zone as created; note soa_edit_api comes back DEFAULT unasked",
			http.MethodPost, "/api/v1/servers/localhost/zones", "",
			map[string]any{
				"name": "fixture.test.", "kind": "Native",
				"nameservers": []string{"ns1.fixture.test."},
			},
		},
		{
			testutil.ProductAuth, auth, "5.1.3", "zone-get",
			"a zone read back, with its rrsets",
			http.MethodGet, "/api/v1/servers/localhost/zones/fixture.test.", "", nil,
		},
		{
			testutil.ProductAuth, auth, "5.1.3", "zone-not-found",
			"the 404 shape for a zone that does not exist",
			http.MethodGet, "/api/v1/servers/localhost/zones/absent.test.", "", nil,
		},
		{
			testutil.ProductAuth, auth, "5.1.3", "view-write-rejected-on-gpgsql",
			"the 422 that the capability classifier turns into the LMDB requirement",
			http.MethodPost, "/api/v1/servers/localhost/views/fixtureview", "",
			map[string]any{"name": "fixture.test."},
		},
		{
			testutil.ProductRec, rec, "5.4.4", "servers-list",
			"the recursor server list",
			http.MethodGet, "/api/v1/servers", "", nil,
		},
		{
			testutil.ProductRec, rec, "5.4.4", "config-allow-from",
			"one of exactly two settings the recursor API will read by name",
			http.MethodGet, "/api/v1/servers/localhost/config/allow-from", "", nil,
		},
		{
			testutil.ProductRec, rec, "5.4.4", "config-unknown-setting",
			"every other setting name answers 404, which is why the resource validates the name",
			http.MethodGet, "/api/v1/servers/localhost/config/max-cache-entries", "", nil,
		},
		{
			testutil.ProductDNSDist, dd, "2.1.0", "server-stats",
			"the dnsdist summary object; note it is not a list, unlike the other two products",
			http.MethodGet, "/api/v1/servers/localhost", "", nil,
		},
		{
			testutil.ProductDNSDist, dd, "2.1.0", "config-allow-from",
			"the one dnsdist setting the API can write",
			http.MethodGet, "/api/v1/servers/localhost/config/allow-from", "", nil,
		},
	}

	client := &http.Client{}
	for _, c := range captures {
		if c.base == "" {
			t.Logf("skipping %s/%s: endpoint not configured", c.product, c.name)
			continue
		}

		var reader io.Reader
		if c.body != nil {
			raw, err := json.Marshal(c.body)
			if err != nil {
				t.Fatalf("%s: encoding: %v", c.name, err)
			}
			reader = bytesReader(raw)
		}

		target := c.base + c.path
		if c.query != "" {
			target += "?" + c.query
		}

		req, err := http.NewRequestWithContext(ctx, c.method, target, reader)
		if err != nil {
			t.Fatalf("%s: request: %v", c.name, err)
		}
		req.Header.Set("X-Api-Key", key)
		if c.body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		f := testutil.Fixture{
			Name:            c.name,
			Description:     c.description,
			Method:          c.method,
			Path:            c.path,
			Query:           c.query,
			Status:          resp.StatusCode,
			RecordedAgainst: c.version,
		}
		if json.Valid(raw) && len(raw) > 0 {
			f.Body = json.RawMessage(compact(t, raw))
		}

		if err := testutil.Save(".", c.product, f); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		t.Logf("recorded %s/%s → %d", c.product, c.name, resp.StatusCode)
	}

	// Leave nothing behind on the server.
	if auth != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
			auth+"/api/v1/servers/localhost/zones/fixture.test.", nil)
		req.Header.Set("X-Api-Key", key)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

func compact(t *testing.T, raw []byte) []byte {
	t.Helper()
	var out []byte
	buf := newBuffer()
	if err := json.Indent(buf, raw, "  ", "  "); err != nil {
		return raw
	}
	out = buf.Bytes()
	return out
}
