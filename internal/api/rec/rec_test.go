package rec_test

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

	"github.com/ioplane/terraform-provider-powerdns/internal/api/rec"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

func newFixtureClient(t *testing.T) (*rec.Client, *testutil.MockServer) {
	t.Helper()

	fixtures, err := testutil.Load("../../testutil", testutil.ProductRec)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no Recursor fixtures; run task fixtures:record")
	}

	mock := testutil.NewMockServer(t, fixtures)
	return rec.New(newTransport(t, mock.URL)), mock
}

type recorded struct {
	Method     string
	RequestURI string
	Query      url.Values
	Body       string
}

func newRecordingClient(t *testing.T) (*rec.Client, *[]recorded) {
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

	return rec.New(newTransport(t, srv.URL)), &seen
}

func newTransport(t *testing.T, baseURL string) *transport.Client {
	t.Helper()

	c, err := transport.New(transport.Config{
		BaseURL:  baseURL,
		APIKey:   "testkey",
		Product:  transport.ProductRecursor,
		Timeout:  2 * time.Second,
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("transport.New: %v", err)
	}
	return c
}

// TestGetSetting_RejectsAnUnregisteredName is the point of having two named
// constants instead of a map-shaped API.
//
// ws-recursor.cc registers allow-from and allow-notify-from as four separate
// handlers, not one parameterised route, so every other name answers 404 — on
// read as well as write. That 404 is indistinguishable from an unreachable
// server, so the client refuses before sending.
func TestGetSetting_RejectsAnUnregisteredName(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	_, err := client.GetSetting(context.Background(), "max-cache-entries")
	if !errors.Is(err, rec.ErrSettingNotWritable) {
		t.Errorf("err = %v, want ErrSettingNotWritable", err)
	}
	if len(*seen) != 0 {
		t.Errorf("the client sent %d request(s); it should refuse before the wire", len(*seen))
	}
	if !strings.Contains(err.Error(), "max-cache-entries") {
		t.Errorf("the rejected name must appear in the error: %v", err)
	}
}

func TestSetSetting_RejectsAnUnregisteredName(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	err := client.SetSetting(context.Background(), "loglevel", []string{"5"})
	if !errors.Is(err, rec.ErrSettingNotWritable) {
		t.Errorf("err = %v, want ErrSettingNotWritable", err)
	}
	if len(*seen) != 0 {
		t.Errorf("the client sent %d request(s), want 0", len(*seen))
	}
}

func TestSetSetting_AcceptsTheTwoRegisteredNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{rec.SettingAllowFrom, rec.SettingAllowNotifyFrom} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, seen := newRecordingClient(t)
			err := client.SetSetting(context.Background(), name, []string{"10.0.0.0/8"})
			if err != nil {
				t.Fatalf("SetSetting(%s): %v", name, err)
			}

			want := "/api/v1/servers/localhost/config/" + name
			if got := (*seen)[0].RequestURI; got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
			if !strings.Contains((*seen)[0].Body, "10.0.0.0/8") {
				t.Errorf("the value did not reach the request: %s", (*seen)[0].Body)
			}
		})
	}
}

// TestGetZone_ServerNormalisesTheUpstream records a normalisation that would
// otherwise be a permanent diff: an upstream given as an address comes back
// with :53 appended.
func TestGetZone_ServerNormalisesTheUpstream(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	zone, err := client.GetZone(context.Background(), "recprobe.test.")
	if err != nil {
		t.Fatalf("GetZone: %v", err)
	}

	if zone.Kind != rec.KindForwarded {
		t.Errorf("Kind = %q, want %q", zone.Kind, rec.KindForwarded)
	}
	if len(zone.Servers) == 0 {
		t.Fatal("a Forwarded zone with no servers")
	}
	if !strings.Contains(zone.Servers[0], ":") {
		t.Errorf("server %q carries no port; the Recursor appends :53 to a bare address, "+
			"and a resource comparing the configured value as a string would diff forever",
			zone.Servers[0])
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost/zones/recprobe.test.")
}

// TestGetZone_RecursionDesiredIsDistinguishable checks the pointer. A plain
// bool cannot tell "the operator asked for false" from "the operator said
// nothing", and for this field the two mean different things upstream.
func TestGetZone_RecursionDesiredIsDistinguishable(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	zone, err := client.GetZone(context.Background(), "recprobe.test.")
	if err != nil {
		t.Fatalf("GetZone: %v", err)
	}
	if zone.RecursionDesired == nil {
		t.Fatal("recursion_desired is absent; the server sends it and the field must decode")
	}
	if *zone.RecursionDesired {
		t.Error("recursion_desired came back true; the fixture was recorded with false")
	}
}

// TestRPZStatistics_EmptyIsNormal records that no response policy zones loaded
// gives an empty object rather than an error.
func TestRPZStatistics_EmptyIsNormal(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	stats, err := client.GetRPZStatistics(context.Background())
	if err != nil {
		t.Fatalf("GetRPZStatistics: %v", err)
	}
	if len(stats) != 0 {
		t.Logf("the lab has %d response policy zone(s) loaded", len(stats))
	}
}

// TestFlushCache_ZeroCountIsSuccess pins the same property as its
// Authoritative twin: nothing cached means nothing to drop, not a failure.
func TestFlushCache_ZeroCountIsSuccess(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	result, err := client.FlushCache(context.Background(), "recprobe.test.")
	if err != nil {
		t.Fatalf("FlushCache: %v", err)
	}
	if result.Result == "" {
		t.Error("the flush reported no result string")
	}
}

func TestGetStatistics(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	stats, err := client.GetStatistics(context.Background())
	if err != nil {
		t.Fatalf("GetStatistics: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("no statistics in the fixture")
	}
}

func TestListServers(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("got %d servers; the Recursor hosts exactly one", len(servers))
	}
	if servers[0].DaemonType != "recursor" {
		t.Errorf("daemon_type = %q, want recursor — this is how a client tells the "+
			"products apart when pointed at the wrong port", servers[0].DaemonType)
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers")
}

func TestZoneIDIsEscaped(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if err := client.DeleteZone(context.Background(), "zone with spaces."); err != nil {
		t.Fatalf("DeleteZone: %v", err)
	}
	if got := (*seen)[0].RequestURI; strings.Contains(got, "zone with spaces") {
		t.Errorf("the zone id reached the wire unescaped: %s", got)
	}
}
