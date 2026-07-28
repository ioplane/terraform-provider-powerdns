//go:build acceptance

package transport_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
)

// TestLive_CapabilityRulesHold verifies against real servers that each
// capability rule still fires on the condition it was derived from.
//
// The unit tests in classify_test.go assert the mapping; this asserts the
// premise. If PowerDNS changes a status code, the unit tests keep passing while
// the provider silently stops explaining itself — this is what catches that.
func TestLive_CapabilityRulesHold(t *testing.T) {
	key := os.Getenv("PDNS_API_KEY")
	if key == "" {
		t.Skip("PDNS_API_KEY not set; run task lab:up")
	}

	ctx := context.Background()

	t.Run("views need LMDB on the SQL backend", func(t *testing.T) {
		url := os.Getenv("PDNS_SERVER_URL")
		if url == "" {
			t.Skip("PDNS_SERVER_URL not set")
		}
		c := mustClient(t, url, key, transport.ProductAuth)

		// The zone must exist for the failure to be about the backend rather
		// than about a missing zone.
		_ = c.Do(ctx, "create zone", http.MethodPost,
			"/api/v1/servers/localhost/zones",
			map[string]any{"name": "cap-probe.test.", "kind": "Native"}, nil)
		t.Cleanup(func() {
			_ = c.Do(ctx, "delete zone", http.MethodDelete,
				"/api/v1/servers/localhost/zones/cap-probe.test.", nil, nil)
		})

		err := c.Do(ctx, "add zone to view", http.MethodPost,
			"/api/v1/servers/localhost/views/capprobe",
			map[string]any{"name": "cap-probe.test."}, nil)

		assertCapability(t, err, transport.CapabilityViewsNeedLMDB)
	})

	t.Run("dnsdist cache flush without a packet cache", func(t *testing.T) {
		url := os.Getenv("PDNS_DNSDIST_SERVER_URL")
		if url == "" {
			t.Skip("PDNS_DNSDIST_SERVER_URL not set")
		}
		c := mustClient(t, url, key, transport.ProductDNSDist)

		// The lab configures a packet cache, so this path succeeds there. The
		// assertion is that a successful flush is NOT misclassified — the
		// mirror image of the rule, and the easier one to get wrong.
		err := c.Do(ctx, "flush cache", http.MethodDelete,
			"/api/v1/cache?pool=&name=absent.example.", nil, nil)

		var apiErr *transport.APIError
		if errors.As(err, &apiErr) && apiErr.Capability == transport.CapabilityDNSDistNoPacketCache {
			t.Error("the lab has a packet cache; this must not be classified as a missing one")
		}
	})
}

func mustClient(t *testing.T, base, key string, p transport.Product) *transport.Client {
	t.Helper()
	c, err := transport.New(transport.Config{BaseURL: base, APIKey: key, Product: p, Attempts: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func assertCapability(t *testing.T, err error, want transport.Capability) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a failure classified as %v, got success", want)
	}
	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err is %T, want *transport.APIError: %v", err, err)
	}
	if apiErr.Capability != want {
		t.Errorf("status %d classified as %v, want %v\nserver said: %q",
			apiErr.StatusCode, apiErr.Capability, want, apiErr.ServerMessage)
	}
}
