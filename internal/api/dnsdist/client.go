// Package dnsdist is the client for the PowerDNS dnsdist API.
//
// Ten operations, eight of which read. Counted from `registerBuiltInWebHandlers`
// in `pdns/dnsdistdist/dnsdist-web.cc` at tag `dnsdist-2.1.0`.
//
// The count needs explaining, because dnsdist is shaped unlike the other two.
// It registers a handler per **path**, not per method-and-path pair, and each
// handler dispatches on the method inside itself. Eight paths are registered;
// `config/allow-from` accepts GET and PUT, and `/api/v1/cache` accepts DELETE.
// That is where ten comes from, and counting registrations alone would give
// eight while losing both writes.
//
// # The ceiling
//
// Two writes. That is the whole of what dnsdist's API permits: the ACL, and a
// cache flush. Rules, pools, downstream servers and dynamic blocks are
// configured in Lua or YAML and are not reachable over HTTP at all — so a
// Terraform provider cannot manage dnsdist's policy, only its ACL and its
// cache. This is documented rather than worked around, because the alternative
// is a resource that looks like it manages dnsdist and does not.
//
// # The two gates
//
// `PUT config/allow-from` needs `setAPIWritable(true, dir)` in the Lua
// configuration; `apiConfigDir` alone is not enough, because `isMethodAllowed`
// checks `d_apiReadWrite` before it looks at the path. Without it the answer is
// 405.
//
// `DELETE /api/v1/cache` is **not** gated the same way — `isMethodAllowed`
// admits it without consulting the flag — but it answers 404 when the pool has
// no packet cache, which reads like a missing endpoint and is not.
//
// The transport classifies both, so the diagnostic names the Lua call or the
// missing cache rather than repeating a status code.
package dnsdist

import "github.com/ioplane/terraform-provider-powerdns/internal/api/transport"

// serverID is the only server dnsdist addresses.
const serverID = "localhost"

// Client issues dnsdist requests.
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
