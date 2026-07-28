//go:build acceptance

// Package provider_test holds the acceptance suite.
//
// Every test here talks to the lab: `task lab:up` first, then `task testacc`.
// They are behind the `acceptance` build tag so `go test ./...` stays fast and
// container-free — the contract tests in internal/api cover what can be
// covered without a server.
package provider_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider"
)

// runID distinguishes one test run from another.
//
// Zone names are global on a PowerDNS server, and a run that fails partway
// leaves its zones behind. Without a per-run suffix the next run collides with
// them and every test reports "Conflict" instead of whatever actually broke.
var runID = strconv.FormatInt(time.Now().UnixNano()%1e9, 10)

// acceptanceZoneName builds a unique, canonical zone name for one test.
func acceptanceZoneName(t *testing.T, purpose string) string {
	t.Helper()
	return fmt.Sprintf("%s-%s.acc.test.", purpose, runID)
}

// acceptancePreCheck fails early with a sentence naming the missing variable,
// rather than letting the first request fail with a connection error.
func acceptancePreCheck(t *testing.T) {
	t.Helper()

	for _, name := range []string{"PDNS_SERVER_URL", "PDNS_API_KEY"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set for acceptance tests; run: task lab:up && task testacc", name)
		}
	}
}

// testAccProviderFactories wires the provider under test.
func testAccProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"powerdns": providerserver.NewProtocol6WithError(provider.New("acceptance")()),
	}
}

// acceptanceAuthClient builds a client for the assertions that check the
// server directly rather than through Terraform state.
//
// CheckDestroy is the reason this exists: state says nothing about whether the
// object actually went away.
func acceptanceAuthClient() (*auth.Client, error) {
	http, err := transport.New(transport.Config{
		BaseURL:  os.Getenv("PDNS_SERVER_URL"),
		APIKey:   os.Getenv("PDNS_API_KEY"),
		Product:  transport.ProductAuth,
		Timeout:  30 * time.Second,
		Attempts: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("building the acceptance client: %w", err)
	}
	return auth.New(http), nil
}

// t0 is a context for the direct-to-server assertions, which run outside a
// test's own context.
func t0() context.Context {
	return context.Background()
}
