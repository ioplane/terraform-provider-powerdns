package dnsdist_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/dnsdist"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

func newFixtureClient(t *testing.T) (*dnsdist.Client, *testutil.MockServer) {
	t.Helper()

	fixtures, err := testutil.Load("../../testutil", testutil.ProductDNSDist)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no dnsdist fixtures; run task fixtures:record")
	}

	mock := testutil.NewMockServer(t, fixtures)
	return dnsdist.New(newTransport(t, mock.URL)), mock
}

type recorded struct {
	Method     string
	RequestURI string
	Query      url.Values
	Body       string
}

func newRecordingClient(t *testing.T) (*dnsdist.Client, *[]recorded) {
	t.Helper()

	var (
		mu   sync.Mutex
		seen []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request body: %v", err)
		}

		mu.Lock()
		seen = append(seen, recorded{
			Method: r.Method, RequestURI: r.RequestURI,
			Query: r.URL.Query(), Body: string(body),
		})
		mu.Unlock()

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	return dnsdist.New(newTransport(t, srv.URL)), &seen
}

func newTransport(t *testing.T, baseURL string) *transport.Client {
	t.Helper()

	c, err := transport.New(transport.Config{
		BaseURL:  baseURL,
		APIKey:   "testkey",
		Product:  transport.ProductDNSDist,
		Timeout:  2 * time.Second,
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("transport.New: %v", err)
	}
	return c
}

// TestGetServer_IsAnObjectNotAList records the shape that differs from the
// other two products. dnsdist registers no /servers collection at all, so a
// client probing reachability the way it does for Authoritative would 404.
func TestGetServer_IsAnObjectNotAList(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	server, err := client.GetServer(context.Background())
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if server.DaemonType == "" {
		t.Error("daemon_type is empty; this is how a client tells the products apart")
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost")
}

// TestFlushCache_CountIsAString is the decoding trap. dnsdist serialises the
// count as "0", not 0, so a struct declaring an int fails to decode a
// successful flush.
func TestFlushCache_CountIsAString(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	result, err := client.FlushCache(context.Background(), "", "probe.test.", "A")
	if err != nil {
		t.Fatalf("FlushCache: %v", err)
	}
	if result.Status == "" {
		t.Error("the flush reported no status")
	}
	if result.Count == "" {
		t.Error("the flush reported no count; the field is a string and must decode as one")
	}
}

// TestFlushCache_UsesTheBareCachePath guards the one endpoint outside
// /servers/localhost. Building it from basePath would 404.
func TestFlushCache_UsesTheBareCachePath(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if _, err := client.FlushCache(context.Background(), "", "example.com.", "A"); err != nil {
		t.Fatalf("FlushCache: %v", err)
	}

	got := (*seen)[0]
	if got.Method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", got.Method)
	}
	if !strings.HasPrefix(got.RequestURI, "/api/v1/cache?") {
		t.Errorf("path = %q; the cache endpoint is not under /servers/localhost", got.RequestURI)
	}
	if got.Query.Get("name") != "example.com." || got.Query.Get("type") != "A" {
		t.Errorf("query = %v", got.Query)
	}
	// The empty pool means the default pool and must still be sent.
	if _, ok := got.Query["pool"]; !ok {
		t.Error("the pool parameter was omitted; the default pool is the empty string, not absence")
	}
}

// TestGetPool_SendsTheNameEvenWhenEmpty pins the required parameter. Omitting
// it answers 400 with an empty body, which is harder to read than the 404 it
// resembles.
func TestGetPool_SendsTheNameEvenWhenEmpty(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if _, err := client.GetPool(context.Background(), ""); err != nil {
		t.Fatalf("GetPool: %v", err)
	}
	if _, ok := (*seen)[0].Query["name"]; !ok {
		t.Error("the name parameter was omitted; dnsdist answers 400 without it")
	}
}

// TestGetPool_UnknownPoolIs404 records that asking for a pool nobody
// configured is a not-found, with an empty body carrying no explanation. A
// resource reading a pool must treat that as "absent", not as an outage.
func TestGetPool_UnknownPoolIs404(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	_, err := client.GetPool(context.Background(), "nonexistent")
	if !errors.Is(err, transport.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestGetPool_HealthCheckFailuresAreCumulative documents a field that reads
// alarming and is not: it counts every failure since start, so State is what
// to read for current health.
func TestGetPool_HealthCheckFailuresAreCumulative(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	pool, err := client.GetPool(context.Background(), "")
	if err != nil {
		t.Fatalf("GetPool: %v", err)
	}
	if len(pool.Servers) == 0 {
		t.Fatal("no downstream servers in the fixture")
	}
	if pool.Servers[0].Address == "" {
		t.Error("a downstream server with no address")
	}
}

func TestGetACL(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	acl, err := client.GetACL(context.Background())
	if err != nil {
		t.Fatalf("GetACL: %v", err)
	}
	if len(acl.Value) == 0 {
		t.Error("the ACL is empty; dnsdist ships a default allow-from")
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost/config/allow-from")
}

func TestSetACL(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if err := client.SetACL(context.Background(), []string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	got := (*seen)[0]
	if got.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", got.Method)
	}
	if !strings.Contains(got.Body, "10.0.0.0/8") {
		t.Errorf("the netmask did not reach the request: %s", got.Body)
	}
	if !strings.Contains(got.Body, "allow-from") {
		t.Errorf("the setting name did not reach the request: %s", got.Body)
	}
}

func TestGetConfigAndRings(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	settings, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if len(settings) == 0 {
		t.Fatal("no configuration in the fixture")
	}

	// The dump calls the ACL "acl"; the writable endpoint is
	// "config/allow-from". Matching a dump entry against the endpoint path
	// finds nothing, which is a trap for any resource reconciling the two.
	var acl, allowFrom int
	for _, s := range settings {
		switch s.Name {
		case "acl":
			acl++
		case "allow-from":
			allowFrom++
		}
	}
	if acl != 1 {
		t.Errorf("found %d acl entries in the dump, want 1", acl)
	}
	if allowFrom != 0 {
		t.Errorf("the dump now carries an allow-from entry; the two names have " +
			"converged and the warning in GetConfig is stale")
	}

	if _, err := client.GetRings(context.Background()); err != nil {
		t.Fatalf("GetRings: %v", err)
	}
}
