package dnsdist_test

import "testing"

// operations is the dnsdist API surface.
//
// From `registerBuiltInWebHandlers` in `pdns/dnsdistdist/dnsdist-web.cc` at
// tag `dnsdist-2.1.0`. The count needs care: dnsdist registers a handler per
// **path** and dispatches on method inside it, so counting registrations gives
// eight and silently loses both writes. What follows is method-and-path pairs,
// matching how the other two products are counted.
//
// Four registrations in that block are compiled out by DISABLE_* guards in
// some builds — `/jsonstat` and `/metrics` by DISABLE_BUILTIN_HTML and
// DISABLE_PROMETHEUS, the two config paths by DISABLE_WEB_CONFIG, and the
// cache path by DISABLE_WEB_CACHE_MANAGEMENT. The lab image has all of them,
// which is what the fixtures were recorded against.
var operations = []struct{ operation, method string }{
	// Reads — 8.
	{"GET /jsonstat", "GetJSONStats"},
	{"GET /metrics", "— Prometheus scrape, not a provider concern"},
	{"GET /api/v1/servers/localhost", "GetServer"},
	{"GET /api/v1/servers/localhost/pool", "GetPool"},
	{"GET /api/v1/servers/localhost/statistics", "GetStatistics"},
	{"GET /api/v1/servers/localhost/rings", "GetRings"},
	{"GET /api/v1/servers/localhost/config", "GetConfig"},
	{"GET /api/v1/servers/localhost/config/allow-from", "GetACL"},

	// Writes — 2. This is the whole of what dnsdist's API permits.
	{"PUT /api/v1/servers/localhost/config/allow-from", "SetACL"},
	{"DELETE /api/v1/cache", "FlushCache"},
}

// dnsdistOperationCount is what dnsdist-web.cc exposes at tag dnsdist-2.1.0.
const dnsdistOperationCount = 10

// writeOperationCount is the number that constrains the whole design: rules,
// pools, downstreams and dynamic blocks are Lua or YAML and are not reachable
// over HTTP, so a provider can manage the ACL and the cache and nothing else.
const writeOperationCount = 2

func TestSurfaceIsComplete(t *testing.T) {
	t.Parallel()

	if len(operations) != dnsdistOperationCount {
		t.Errorf("the surface lists %d operations, and dnsdist 2.1.0 exposes %d",
			len(operations), dnsdistOperationCount)
	}

	var writes int
	seen := make(map[string]bool, len(operations))
	for _, op := range operations {
		if op.method == "" {
			t.Errorf("%s has no client method", op.operation)
		}
		if seen[op.operation] {
			t.Errorf("%s is listed twice", op.operation)
		}
		seen[op.operation] = true

		switch {
		case op.operation[:3] == "PUT", op.operation[:6] == "DELETE":
			writes++
		}
	}

	if writes != writeOperationCount {
		t.Errorf("the surface has %d write operations, want %d. If this grew, dnsdist "+
			"gained an HTTP write path and ADR 0006 needs revisiting", writes, writeOperationCount)
	}
}
