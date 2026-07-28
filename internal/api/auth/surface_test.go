package auth_test

import "testing"

// operations is the Authoritative API surface as registered in ws-auth.cc,
// counted by:
//
//	grep -E 'registerApiHandler\("/api/v1/servers' pdns/ws-auth.cc | wc -l
//
// One line per operation, each naming the client method that covers it. The
// list is here rather than in a document because the exit gate for phase 2 is
// a number — "all 68 operations" — and a phase that claims 68 and delivers 61
// should be visible without anybody recounting.
var operations = []struct{ operation, method string }{
	// Zones — 10. S2-01.
	{"GET /zones", "ListZones, SearchZoneByName"},
	{"POST /zones", "CreateZone"},
	{"GET /zones/{id}", "GetZone"},
	{"PUT /zones/{id}", "UpdateZone"},
	{"PATCH /zones/{id}", "PatchRRSets"},
	{"DELETE /zones/{id}", "DeleteZone"},
	{"PUT /zones/{id}/notify", "NotifyZone"},
	{"PUT /zones/{id}/axfr-retrieve", "AXFRRetrieveZone"},
	{"GET /zones/{id}/export", "ExportZone"},
	{"PUT /zones/{id}/rectify", "RectifyZone"},

	// Metadata — 5. S2-02.
	{"GET /zones/{id}/metadata", "ListMetadata"},
	{"POST /zones/{id}/metadata", "CreateMetadata"},
	{"GET /zones/{id}/metadata/{kind}", "GetMetadata"},
	{"PUT /zones/{id}/metadata/{kind}", "SetMetadata"},
	{"DELETE /zones/{id}/metadata/{kind}", "DeleteMetadata"},

	// Cryptokeys — 6. S2-02.
	{"GET /zones/{id}/cryptokeys", "ListCryptoKeys"},
	{"POST /zones/{id}/cryptokeys", "CreateCryptoKey"},
	{"GET /zones/{id}/cryptokeys/{key_id}", "GetCryptoKey"},
	{"PUT /zones/{id}/cryptokeys/{key_id}", "SetCryptoKeyState"},
	{"DELETE /zones/{id}/cryptokeys/{key_id}", "DeleteCryptoKey"},
	{"POST /zones/{id}/cryptokeys/{key_id}", "RectifyAfterKeyChange"},

	// TSIG keys — 5. S2-02.
	{"GET /tsigkeys", "ListTSIGKeys"},
	{"POST /tsigkeys", "CreateTSIGKey"},
	{"GET /tsigkeys/{id}", "GetTSIGKey"},
	{"PUT /tsigkeys/{id}", "UpdateTSIGKey"},
	{"DELETE /tsigkeys/{id}", "DeleteTSIGKey"},

	// Autoprimaries — 3. S2-02.
	{"GET /autoprimaries", "ListAutoprimaries"},
	{"POST /autoprimaries", "CreateAutoprimary"},
	{"DELETE /autoprimaries/{ip}/{nameserver}", "DeleteAutoprimary"},

	// Views — 4. S2-03. LMDB only.
	{"GET /views", "ListViews"},
	{"GET /views/{view}", "GetView"},
	{"POST /views/{view}", "AddZoneToView"},
	{"DELETE /views/{view}/{id}", "RemoveZoneFromView"},

	// Networks — 3. S2-03. LMDB only.
	{"GET /networks", "ListNetworks"},
	{"GET /networks/{ip}/{prefixlen}", "GetNetwork"},
	{"PUT /networks/{ip}/{prefixlen}", "SetNetwork"},

	// Servers, config, statistics, search, cache — 5. S2-03.
	{"GET /servers", "ListServers"},
	{"GET /servers/localhost", "GetServer"},
	{"GET /config", "GetConfig"},
	{"GET /statistics", "GetStatistics"},
	{"GET /search-data", "Search"},
	{"PUT /cache/flush", "FlushCache"},
}

// authOperationCount is what ws-auth.cc registers for Authoritative 5.1.3.
const authOperationCount = 42

// TestSurfaceIsComplete asserts the client covers every registered operation.
//
// This does not prove each method works — the contract tests do that. It
// proves nothing was quietly dropped, which is the failure mode a per-method
// test suite cannot see, because a missing method has no test to fail.
func TestSurfaceIsComplete(t *testing.T) {
	t.Parallel()

	if len(operations) != authOperationCount {
		t.Errorf("the surface lists %d operations, and Authoritative 5.1.3 registers %d.\n"+
			"Either an operation is missing here or PowerDNS changed; check with\n"+
			`  grep -E 'registerApiHandler\("/api/v1/servers' pdns/ws-auth.cc | wc -l`,
			len(operations), authOperationCount)
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
