package testutil_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

// TestMockServerReplaysRecordedResponses is the contract layer working
// end to end: real PowerDNS payloads, a real client, no containers.
func TestMockServerReplaysRecordedResponses(t *testing.T) {
	t.Parallel()

	fixtures, err := testutil.Load(".", testutil.ProductAuth)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fixtures) == 0 {
		t.Skip("no fixtures recorded yet; run task fixtures:record")
	}

	mock := testutil.NewMockServer(t, fixtures)
	client, err := transport.New(transport.Config{
		BaseURL:  mock.URL,
		APIKey:   "testkey",
		Product:  transport.ProductAuth,
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()

	var servers []struct {
		ID         string `json:"id"`
		DaemonType string `json:"daemon_type"`
		Version    string `json:"version"`
	}
	if err := client.Do(ctx, "list servers", http.MethodGet, "/api/v1/servers", nil, &servers); err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if len(servers) != 1 || servers[0].DaemonType != "authoritative" {
		t.Errorf("unexpected server list: %+v", servers)
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers")
}

// TestMockServerSurfacesTheRecordedErrors proves the recorded failure shapes
// still classify the way the transport expects. A change in what PowerDNS
// sends shows up here as soon as the fixture is re-recorded.
func TestMockServerSurfacesTheRecordedErrors(t *testing.T) {
	t.Parallel()

	fixtures, err := testutil.Load(".", testutil.ProductAuth)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fixtures) == 0 {
		t.Skip("no fixtures recorded yet")
	}

	mock := testutil.NewMockServer(t, fixtures)
	client, err := transport.New(transport.Config{
		BaseURL:  mock.URL,
		APIKey:   "testkey",
		Product:  transport.ProductAuth,
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()

	t.Run("a missing zone is ErrNotFound", func(t *testing.T) {
		t.Parallel()

		err := client.Do(ctx, "get zone", http.MethodGet,
			"/api/v1/servers/localhost/zones/absent.test.", nil, nil)
		if !errors.Is(err, transport.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("a view write on gpgsql names the LMDB requirement", func(t *testing.T) {
		t.Parallel()

		err := client.Do(ctx, "add zone to view", http.MethodPost,
			"/api/v1/servers/localhost/views/fixtureview", nil, nil)

		var apiErr *transport.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("err is %T, want *transport.APIError", err)
		}
		if apiErr.Capability != transport.CapabilityViewsNeedLMDB {
			t.Errorf("Capability = %v, want %v — the recorded 422 stopped classifying",
				apiErr.Capability, transport.CapabilityViewsNeedLMDB)
		}
	})
}

// TestMockServerRejectsAWrongKey covers the mock's own behaviour: a client
// sending the wrong key must see a 401 rather than a fixture.
func TestMockServerRejectsAWrongKey(t *testing.T) {
	t.Parallel()

	fixtures, err := testutil.Load(".", testutil.ProductAuth)
	if err != nil || len(fixtures) == 0 {
		t.Skip("no fixtures recorded yet")
	}

	mock := testutil.NewMockServer(t, fixtures)
	client, err := transport.New(transport.Config{
		BaseURL:  mock.URL,
		APIKey:   "wrong",
		Product:  transport.ProductAuth,
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = client.Do(context.Background(), "list servers", http.MethodGet, "/api/v1/servers", nil, nil)
	if !errors.Is(err, transport.ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}
