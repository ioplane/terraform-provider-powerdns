package transport

import (
	"net/http"
	"testing"
)

// TestClassify covers every capability rule and, as importantly, the cases
// that must NOT be classified. A rule that fires too eagerly is worse than one
// that does not fire: it tells an operator to change a backend setting when the
// real problem is a typo in a zone name.
func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		product Product
		status  int
		path    string
		message string
		want    Capability
	}{
		// Authoritative: views and networks need LMDB.
		{
			name:    "auth view write on a SQL backend",
			product: ProductAuth,
			status:  http.StatusUnprocessableEntity,
			path:    "/api/v1/servers/localhost/views/trusted",
			message: "Failed to add example.com. to view trusted",
			want:    CapabilityViewsNeedLMDB,
		},
		{
			name:    "auth network write on a SQL backend",
			product: ProductAuth,
			status:  http.StatusUnprocessableEntity,
			path:    "/api/v1/servers/localhost/networks/10.0.0.0/8",
			want:    CapabilityViewsNeedLMDB,
		},
		{
			name:    "auth view not found is not a backend limit",
			product: ProductAuth,
			status:  http.StatusNotFound,
			path:    "/api/v1/servers/localhost/views/trusted",
			want:    CapabilityNone,
		},
		{
			name:    "auth unauthorized on views is not a backend limit",
			product: ProductAuth,
			status:  http.StatusUnauthorized,
			path:    "/api/v1/servers/localhost/views/trusted",
			want:    CapabilityNone,
		},
		{
			name:    "auth 422 on a zone is an ordinary rejection",
			product: ProductAuth,
			status:  http.StatusUnprocessableEntity,
			path:    "/api/v1/servers/localhost/zones",
			message: "Conflict",
			want:    CapabilityNone,
		},

		// Recursor: writes need api-config-dir. Keyed on the message, because
		// the Recursor names the setting itself.
		{
			name:    "recursor write without api-config-dir",
			product: ProductRecursor,
			status:  http.StatusUnprocessableEntity,
			path:    "/api/v1/servers/localhost/zones",
			message: `Config Option "api-config-dir" must be set`,
			want:    CapabilityRecursorNeedsAPIDir,
		},
		{
			name:    "recursor 422 for another reason",
			product: ProductRecursor,
			status:  http.StatusUnprocessableEntity,
			path:    "/api/v1/servers/localhost/zones",
			message: "kind=Native and recursion_desired are mutually exclusive",
			want:    CapabilityNone,
		},

		// dnsdist: the write gate, and the cache case.
		{
			name:    "dnsdist write without setAPIWritable",
			product: ProductDNSDist,
			status:  http.StatusMethodNotAllowed,
			path:    "/api/v1/servers/localhost/config/allow-from",
			want:    CapabilityDNSDistNotWritable,
		},
		{
			name:    "dnsdist cache flush with no packet cache",
			product: ProductDNSDist,
			status:  http.StatusNotFound,
			path:    "/api/v1/cache",
			want:    CapabilityDNSDistNoPacketCache,
		},
		{
			name:    "dnsdist 404 elsewhere is an ordinary not-found",
			product: ProductDNSDist,
			status:  http.StatusNotFound,
			path:    "/api/v1/servers/localhost/pool",
			want:    CapabilityNone,
		},

		// A 405 from the Authoritative server is a plain method error: the
		// dnsdist rule must not leak across products.
		{
			name:    "auth 405 is not the dnsdist write gate",
			product: ProductAuth,
			status:  http.StatusMethodNotAllowed,
			path:    "/api/v1/servers/localhost/zones",
			want:    CapabilityNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classify(tt.product, tt.status, tt.path, tt.message)
			if got != tt.want {
				t.Errorf("classify(%s, %d, %q, %q) = %v, want %v",
					tt.product, tt.status, tt.path, tt.message, got, tt.want)
			}
		})
	}
}

// TestCapabilityRequirement checks that every classified capability produces an
// explanation. A classification with no requirement text would reach the
// operator as a bare status, which is the failure the type exists to prevent.
func TestCapabilityRequirement(t *testing.T) {
	t.Parallel()

	classified := []Capability{
		CapabilityViewsNeedLMDB,
		CapabilityRecursorNeedsAPIDir,
		CapabilityDNSDistNotWritable,
		CapabilityDNSDistNoPacketCache,
	}

	for _, c := range classified {
		if c.Requirement() == "" {
			t.Errorf("%v has no requirement text", c)
		}
		if c.String() == "unknown" {
			t.Errorf("%v has no name", c)
		}
	}

	if CapabilityNone.Requirement() != "" {
		t.Error("CapabilityNone must produce no requirement text")
	}
}
