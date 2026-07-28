package rec_test

import "testing"

// operations is the Recursor API surface.
//
// Counted from the registration block in `pdns/recursordist/ws-recursor.cc` at
// tag `rec-5.4.4`, lines 873 to 888 — sixteen consecutive
// `registerApiHandler` calls under `/api/v1/servers`.
//
// Four handlers in that same block are deliberately excluded, and the reason
// differs for each:
//
//   - `/jsonstat` is registered above them as "legacy dispatch"
//   - `/api` and `/api/v1` are discovery endpoints describing the API
//   - `/metrics` is a `registerWebHandler`, not an API handler at all
//
// PowerDNS publishes no OpenAPI document for the Recursor, so this list is not
// checkable against a specification. The source is the specification.
var operations = []struct{ operation, method string }{
	// Zones — 5.
	{"GET /zones", "ListZones"},
	{"POST /zones", "CreateZone"},
	{"GET /zones/{id}", "GetZone"},
	{"PUT /zones/{id}", "UpdateZone"},
	{"DELETE /zones/{id}", "DeleteZone"},

	// Config — 5. Only two names are writable, and only those two are
	// readable by name; everything else lives in the whole-config dump.
	{"GET /config", "GetConfig"},
	{"GET /config/allow-from", "GetSetting"},
	{"PUT /config/allow-from", "SetSetting"},
	{"GET /config/allow-notify-from", "GetSetting"},
	{"PUT /config/allow-notify-from", "SetSetting"},

	// Statistics — 2.
	{"GET /statistics", "GetStatistics"},
	{"GET /rpzstatistics", "GetRPZStatistics"},

	// Search — 1.
	{"GET /search-data", "Search"},

	// Cache — 1.
	{"PUT /cache/flush", "FlushCache"},

	// Servers — 2.
	{"GET /servers", "ListServers"},
	{"GET /servers/localhost", "GetServer"},
}

// recOperationCount is what ws-recursor.cc registers under /api/v1/servers at
// tag rec-5.4.4.
const recOperationCount = 16

func TestSurfaceIsComplete(t *testing.T) {
	t.Parallel()

	if len(operations) != recOperationCount {
		t.Errorf("the surface lists %d operations, and Recursor 5.4.4 registers %d",
			len(operations), recOperationCount)
	}

	seen := make(map[string]bool, len(operations))
	for _, op := range operations {
		if op.method == "" {
			t.Errorf("%s has no client method", op.operation)
		}
		if seen[op.operation] {
			t.Errorf("%s is listed twice", op.operation)
		}
		seen[op.operation] = true
	}
}
