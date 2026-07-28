package auth_test

import (
	"context"
	"net/http"
	"testing"
)

// TestListViews_UnwrapsTheEnvelope pins a shape that differs from the rest of
// the API: views and networks answer with an object wrapping a list, where
// every other collection answers with a bare array.
func TestListViews_UnwrapsTheEnvelope(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	views, err := client.ListViews(context.Background())
	if err != nil {
		t.Fatalf("ListViews: %v", err)
	}
	if len(views) == 0 {
		t.Fatal("no views in the fixture; the envelope was probably not unwrapped")
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost/views")
}

func TestListNetworks_UnwrapsTheEnvelope(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	networks, err := client.ListNetworks(context.Background())
	if err != nil {
		t.Fatalf("ListNetworks: %v", err)
	}
	if len(networks) == 0 {
		t.Fatal("no networks in the fixture")
	}
	if networks[0].Network == "" || networks[0].View == "" {
		t.Errorf("incomplete mapping: %+v", networks[0])
	}
}

// TestNetworkPathSplitsTheCIDR is the guard for the awkward path shape: the
// API spells a network as two segments, so escaping the CIDR whole would
// encode the slash and 404.
func TestNetworkPathSplitsTheCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cidr string
		want string
	}{
		{"192.0.2.0/24", "/api/v1/servers/localhost/networks/192.0.2.0/24"},
		{"2001:db8::/32", "/api/v1/servers/localhost/networks/2001:db8::/32"},
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			t.Parallel()

			client, seen := newRecordingClient(t)
			if err := client.SetNetwork(context.Background(), tt.cidr, "trusted"); err != nil {
				t.Fatalf("SetNetwork: %v", err)
			}
			if got := (*seen)[0].RequestURI; got != tt.want {
				t.Errorf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSearch_ReturnsAHeterogeneousList records that one call answers with two
// different shapes. A zone hit carries no content or ttl; a record hit does.
func TestSearch_ReturnsAHeterogeneousList(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	results, err := client.Search(context.Background(), "s202*", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no search results in the fixture")
	}

	var zones, records int
	for _, r := range results {
		switch r.ObjectType {
		case "zone":
			zones++
			if r.Content != "" {
				t.Errorf("a zone hit carried content: %+v", r)
			}
		case "record":
			records++
			if r.Content == "" {
				t.Errorf("a record hit carried no content: %+v", r)
			}
		}
	}
	if zones == 0 || records == 0 {
		t.Errorf("the fixture should cover both shapes; got %d zone(s) and %d record(s)",
			zones, records)
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
	for _, s := range stats {
		if s.Name == "" {
			t.Errorf("a statistic has no name: %+v", s)
		}
	}
}

func TestGetConfig(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	settings, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if len(settings) == 0 {
		t.Fatal("no configuration in the fixture")
	}
}

// TestSearch_EncodesTheQuery guards the wildcard, which is the whole point of
// the endpoint and is not URL-safe.
func TestSearch_EncodesTheQuery(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if _, err := client.Search(context.Background(), "*.example.com", 10); err != nil {
		t.Fatalf("Search: %v", err)
	}

	q := (*seen)[0].Query
	if got := q.Get("q"); got != "*.example.com" {
		t.Errorf("q = %q, want *.example.com", got)
	}
	if got := q.Get("max"); got != "10" {
		t.Errorf("max = %q, want 10", got)
	}
}

// TestFlushCache_ZeroIsNotAFailure records that a flush which dropped nothing
// is a successful flush.
func TestFlushCache_ZeroIsNotAFailure(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	// The recording server answers 204, so the result is the zero value —
	// which is exactly the case being pinned: no error.
	if _, err := client.FlushCache(context.Background(), "example.com."); err != nil {
		t.Fatalf("FlushCache: %v", err)
	}
	if got := (*seen)[0].Query.Get("domain"); got != "example.com." {
		t.Errorf("domain = %q, want example.com.", got)
	}
}
