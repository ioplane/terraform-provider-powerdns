// Package rec is the client for the PowerDNS Recursor API.
//
// Sixteen operations, counted from the registration block in
// `pdns/recursordist/ws-recursor.cc` at tag `rec-5.4.4` — lines 873 to 888.
// PowerDNS publishes no OpenAPI document for the Recursor, so the source is
// the only specification there is, and the contract tests here check recorded
// responses for well-formedness rather than against a schema.
//
// Three handlers in that same block are deliberately out of scope. `/jsonstat`
// is registered as legacy dispatch, and `/api` and `/api/v1` are discovery
// endpoints that describe the API rather than doing anything with it.
// `/metrics` is a `registerWebHandler`, not an API handler at all.
//
// The Recursor is read-only out of the box. Every write — zones and the two
// writable settings — answers 422 `Config Option "api-config-dir" must be set`
// until `webservice.api_dir` is configured, and the transport classifies that
// as CapabilityRecursorNeedsAPIDir so the diagnostic names the setting.
package rec

import (
	"net/url"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
)

// serverID is the only server the Recursor addresses, as in Authoritative.
const serverID = "localhost"

// Client issues Recursor requests.
type Client struct {
	http *transport.Client
}

// New wraps a transport client.
func New(http *transport.Client) *Client {
	return &Client{http: http}
}

// basePath is `/api/v1/servers/localhost`.
func basePath() string {
	return "/api/v1/servers/" + serverID
}

// zonePath escapes a zone id into a path segment.
func zonePath(zoneID string) string {
	return basePath() + "/zones/" + url.PathEscape(zoneID)
}
