package auth_test

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

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

// newFixtureClient wires the auth client to a mock replaying the recorded
// Authoritative fixtures. No containers: the lab is needed to record a
// fixture, not to use one.
//
// The mock fails the test on a request nobody recorded, so this is only for
// calls a fixture covers. Path-construction checks use newRecordingClient.
func newFixtureClient(t *testing.T) (*auth.Client, *testutil.MockServer) {
	t.Helper()

	fixtures, err := testutil.Load("../../testutil", testutil.ProductAuth)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no Authoritative fixtures; run task fixtures:record")
	}

	mock := testutil.NewMockServer(t, fixtures)
	return auth.New(newTransport(t, mock.URL)), mock
}

// newRecordingClient answers everything with 200 {} and records what it was
// asked for. It exists for the assertions that are about the request the
// client built rather than the response it got back.
func newRecordingClient(t *testing.T) (*auth.Client, *[]recorded) {
	t.Helper()

	var (
		mu   sync.Mutex
		seen []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body here: it is closed once the handler returns, and
		// r.Clone does not give a second readable copy.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request body: %v", err)
		}

		mu.Lock()
		seen = append(seen, recorded{
			Method: r.Method, RequestURI: r.RequestURI, Query: r.URL.Query(), Body: string(body),
		})
		mu.Unlock()

		// 204 rather than a body: the transport skips decoding entirely, so
		// one handler serves calls returning an object, a slice and nothing.
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	return auth.New(newTransport(t, srv.URL)), &seen
}

// recorded is what the recording server saw, captured while the request was
// still readable.
type recorded struct {
	Method     string
	RequestURI string
	Query      url.Values
	Body       string
}

func newTransport(t *testing.T, baseURL string) *transport.Client {
	t.Helper()

	c, err := transport.New(transport.Config{
		BaseURL:  baseURL,
		APIKey:   "testkey",
		Product:  transport.ProductAuth,
		Timeout:  2 * time.Second,
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("transport.New: %v", err)
	}
	return c
}

func TestGetZone(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	zone, err := client.GetZone(context.Background(), "fixture.test.")
	if err != nil {
		t.Fatalf("GetZone: %v", err)
	}

	if zone.Name != "fixture.test." {
		t.Errorf("Name = %q, want fixture.test.", zone.Name)
	}
	if zone.Kind != auth.KindNative {
		t.Errorf("Kind = %q, want %q", zone.Kind, auth.KindNative)
	}
	if len(zone.RRSets) == 0 {
		t.Error("a zone read must carry its rrsets")
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost/zones/fixture.test.")
}

func TestGetZone_NotFound(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	_, err := client.GetZone(context.Background(), "absent.test.")
	if !errors.Is(err, transport.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestCreateZone_ReturnsTheServersVersion is the regression guard for the
// normalisation that would otherwise produce a permanent diff: soa_edit_api
// comes back DEFAULT although the create never asked for it.
func TestCreateZone_ReturnsTheServersVersion(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	zone, err := client.CreateZone(context.Background(), auth.Zone{
		Name: "fixture.test.",
		Kind: "native",
	})
	if err != nil {
		t.Fatalf("CreateZone: %v", err)
	}

	if zone.Kind != auth.KindNative {
		t.Errorf("Kind = %q; the server title-cases it and the caller must use what came back",
			zone.Kind)
	}
	if zone.SOAEditAPI == "" {
		t.Error("soa_edit_api came back empty; the server assigns DEFAULT, and a caller " +
			"comparing it against an empty configuration would diff forever")
	}
}

func TestExportZone_UnwrapsTheJSONEnvelope(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	zoneFile, err := client.ExportZone(context.Background(), "exportprobe.test.")
	if err != nil {
		t.Fatalf("ExportZone: %v", err)
	}
	if zoneFile == "" {
		t.Fatal("the export is empty")
	}
	// The point: the endpoint answers JSON with the zone file as a string, not
	// text/plain as "AXFR format" in the documentation implies. A client that
	// took the body verbatim would hand back the envelope.
	if strings.HasPrefix(zoneFile, "{") {
		t.Errorf("the JSON envelope leaked into the result: %q", zoneFile)
	}
	if !strings.Contains(zoneFile, "SOA") {
		t.Errorf("the export carries no SOA: %q", zoneFile)
	}
}

func TestNotifyAndRectify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*auth.Client) error
		path string
	}{
		{
			"notify",
			func(c *auth.Client) error {
				return c.NotifyZone(context.Background(), "exportprobe.test.")
			},
			"/api/v1/servers/localhost/zones/exportprobe.test./notify",
		},
		{
			"rectify",
			func(c *auth.Client) error {
				return c.RectifyZone(context.Background(), "exportprobe.test.")
			},
			"/api/v1/servers/localhost/zones/exportprobe.test./rectify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, mock := newFixtureClient(t)
			if err := tt.call(client); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			mock.AssertCalled(http.MethodPut, tt.path)
		})
	}
}

// TestAXFRRetrieve_OnANonSlaveZone asserts the server's own explanation
// survives into the error. A bare 422 reaching an operator is the defect this
// provider exists to avoid.
func TestAXFRRetrieve_OnANonSlaveZone(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	err := client.AXFRRetrieveZone(context.Background(), "exportprobe.test.")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, transport.ErrRejected) {
		t.Errorf("err = %v, want ErrRejected", err)
	}
	if !strings.Contains(err.Error(), "not a secondary domain") {
		t.Errorf("the server's explanation must survive into the error, got: %v", err)
	}
}

// TestPatchRRSets_RejectsAMissingChangeType checks the client-side guard.
// PowerDNS names the field itself, so this saves a round trip rather than
// improving the message — but it saves it before anything is sent.
func TestPatchRRSets_RejectsAMissingChangeType(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	err := client.PatchRRSets(context.Background(), "fixture.test.", []auth.RRSet{{
		Name:    "www.fixture.test.",
		Type:    "A",
		TTL:     300,
		Records: []auth.Record{{Content: "192.0.2.1"}},
	}})

	if !errors.Is(err, auth.ErrMissingChangeType) {
		t.Errorf("err = %v, want ErrMissingChangeType", err)
	}
	if n := len(mock.Requests()); n != 0 {
		t.Errorf("the client sent %d request(s); a malformed patch must not reach the server", n)
	}
}

func TestPatchRRSets_RejectsAnUnknownChangeType(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	err := client.PatchRRSets(context.Background(), "fixture.test.", []auth.RRSet{{
		Name: "www.fixture.test.", Type: "A", ChangeType: "UPSERT",
	}})
	if !errors.Is(err, auth.ErrMissingChangeType) {
		t.Errorf("err = %v, want ErrMissingChangeType", err)
	}
}

// TestPatchRRSets_EmptyIsANoOp spares every caller a guard of its own. A patch
// with nothing in it is a successful patch, not a 422.
func TestPatchRRSets_EmptyIsANoOp(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	if err := client.PatchRRSets(context.Background(), "fixture.test.", nil); err != nil {
		t.Fatalf("an empty patch must not be an error: %v", err)
	}
	if n := len(mock.Requests()); n != 0 {
		t.Errorf("an empty patch sent %d request(s), want 0", n)
	}
}

// TestZoneIDIsEscaped guards the path construction. A zone id is a canonical
// name and may hold characters that are not path-safe; concatenation works
// until the first one that is not, then fails as a 404 indistinguishable from
// a genuinely missing zone.
func TestZoneIDIsEscaped(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if _, err := client.GetZone(context.Background(), "zone with spaces."); err != nil {
		t.Fatalf("GetZone: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("saw %d requests, want 1", len(*seen))
	}
	// RequestURI is the raw line as sent; r.URL.Path is already decoded, so
	// asserting on that would pass even for an unescaped client.
	if got := (*seen)[0].RequestURI; strings.Contains(got, "zone with spaces") {
		t.Errorf("the zone id reached the wire unescaped: %s", got)
	}
}

// TestUpdateZone_DropsRRSets guards a silent no-op: PowerDNS ignores rrsets in
// a PUT, so a caller who put records there would see success and no change.
func TestUpdateZone_DropsRRSets(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	err := client.UpdateZone(context.Background(), "fixture.test.", auth.Zone{
		Kind:   auth.KindNative,
		RRSets: []auth.RRSet{{Name: "www.fixture.test.", Type: "A"}},
	})
	if err != nil {
		t.Fatalf("UpdateZone: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("saw %d requests, want 1", len(*seen))
	}
	body := (*seen)[0].Body
	if strings.Contains(body, "rrsets") {
		t.Errorf("rrsets reached a PUT, where PowerDNS ignores them: %s", body)
	}
	if !strings.Contains(body, auth.KindNative) {
		t.Errorf("the zone attributes did not reach the request: %s", body)
	}
}

func TestSearchZoneByName_EscapesTheQuery(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if _, err := client.SearchZoneByName(context.Background(), "a b.test."); err != nil {
		t.Fatalf("SearchZoneByName: %v", err)
	}
	if got := (*seen)[0].Query.Get("zone"); got != "a b.test." {
		t.Errorf("zone query = %q, want %q", got, "a b.test.")
	}
}
