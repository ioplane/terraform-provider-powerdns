// Package auth is the client for the PowerDNS Authoritative Server API.
//
// It owns the 42 operations Authoritative registers, and nothing else. The
// three products get three packages rather than one because their APIs only
// look alike: the same path can mean different things, and a shared client
// would have to carry a product flag into every method to keep them apart.
//
// Every method takes a context and returns either a typed value or an error
// from internal/api/transport, which has already classified the failure. A
// caller here never inspects a status code.
package auth

import (
	"net/url"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
)

// serverID is the only server PowerDNS addresses.
//
// The API is shaped as though a daemon could host several — `/servers/{id}` —
// but `ws-auth.cc` answers 404 for any id other than `localhost`. Threading a
// configurable server id through every call would model a flexibility that
// does not exist.
const serverID = "localhost"

// Client issues Authoritative requests.
type Client struct {
	http *transport.Client
}

// New wraps a transport client.
//
// The transport is built by the provider, once, from the operator's
// configuration; this type adds paths and shapes, never policy.
func New(http *transport.Client) *Client {
	return &Client{http: http}
}

// basePath is `/api/v1/servers/localhost`.
func basePath() string {
	return "/api/v1/servers/" + serverID
}

// zonePath escapes a zone id into a path segment.
//
// A zone id is a canonical name — "example.com." — and can contain characters
// that are not path-safe once internationalised names are in play. Building
// the path by concatenation works until the first zone that needs escaping,
// and then fails as a 404 that looks like a missing zone.
func zonePath(zoneID string) string {
	return basePath() + "/zones/" + url.PathEscape(zoneID)
}
