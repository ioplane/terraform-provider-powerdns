package normalise_test

import (
	"testing"
	"unicode"

	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
)

func TestUnicode17Tables(t *testing.T) {
	t.Parallel()
	const tolongSikiLetterA = '\U00011DB4'
	if !unicode.IsLetter(tolongSikiLetterA) {
		t.Fatal("U+11DB4 must be classified as a letter by Unicode 17")
	}
	if !normalise.DNSName("\U00011DB4.example", "\U00011DB4.example.") {
		t.Fatal("DNSName must preserve the new letter and apply the trailing-dot rule")
	}
}

// Each case here is either a normalisation observed on a live PowerDNS server
// — the `same` cases — or a genuine difference that must survive comparison —
// the `differ` cases. The second kind matters as much as the first: a
// comparison looser than the server's hides a real change, which is worse than
// a spurious diff because nobody sees it.

func TestZoneKind(t *testing.T) {
	t.Parallel()

	same := [][2]string{
		{"native", "Native"},
		{"NATIVE", "Native"},
		{"Master", "Master"},
	}
	differ := [][2]string{
		{"Native", "Master"},
		{"Slave", "Producer"},
	}

	for _, c := range same {
		if !normalise.ZoneKind(c[0], c[1]) {
			t.Errorf("ZoneKind(%q, %q) = false; the server title-cases what it is given", c[0], c[1])
		}
	}
	for _, c := range differ {
		if normalise.ZoneKind(c[0], c[1]) {
			t.Errorf("ZoneKind(%q, %q) = true; these are different kinds", c[0], c[1])
		}
	}
}

func TestDNSName(t *testing.T) {
	t.Parallel()

	same := [][2]string{
		{"example.com", "example.com."},
		{"Example.COM.", "example.com."},
		{"www.example.com.", "WWW.example.com"},
	}
	differ := [][2]string{
		{"example.com.", "example.net."},
		{"www.example.com.", "example.com."},
	}

	for _, c := range same {
		if !normalise.DNSName(c[0], c[1]) {
			t.Errorf("DNSName(%q, %q) = false", c[0], c[1])
		}
	}
	for _, c := range differ {
		if normalise.DNSName(c[0], c[1]) {
			t.Errorf("DNSName(%q, %q) = true", c[0], c[1])
		}
	}
}

func TestIPAddress(t *testing.T) {
	t.Parallel()

	same := [][2]string{
		// The compression PowerDNS applies to a zone's master list.
		{"2001:db8:0:0:0:0:0:1", "2001:db8::1"},
		{"2001:0db8::0001", "2001:db8::1"},
		{"192.0.2.1", "192.0.2.1"},
	}
	differ := [][2]string{
		{"192.0.2.1", "192.0.2.2"},
		{"2001:db8::1", "2001:db8::2"},
		// Not an address on either side: compared exactly, so a typo in a
		// hostname is still a diff.
		{"ns1.example.com", "ns2.example.com"},
	}

	for _, c := range same {
		if !normalise.IPAddress(c[0], c[1]) {
			t.Errorf("IPAddress(%q, %q) = false", c[0], c[1])
		}
	}
	for _, c := range differ {
		if normalise.IPAddress(c[0], c[1]) {
			t.Errorf("IPAddress(%q, %q) = true", c[0], c[1])
		}
	}
}

// TestUpstreamServer covers the Recursor's port default, recorded from
// rec-5.4.4: an upstream given as 192.0.2.53 reads back 192.0.2.53:53.
func TestUpstreamServer(t *testing.T) {
	t.Parallel()

	same := [][2]string{
		{"192.0.2.53", "192.0.2.53:53"},
		{"192.0.2.53:53", "192.0.2.53"},
		{"[2001:db8::1]:53", "2001:db8::1"},
		{"2001:db8:0::1", "[2001:db8::1]:53"},
	}
	differ := [][2]string{
		// An explicit non-default port is a real difference.
		{"192.0.2.53", "192.0.2.53:5353"},
		{"192.0.2.53:53", "192.0.2.54:53"},
	}

	for _, c := range same {
		if !normalise.UpstreamServer(c[0], c[1]) {
			t.Errorf("UpstreamServer(%q, %q) = false; the Recursor defaults the port to 53",
				c[0], c[1])
		}
	}
	for _, c := range differ {
		if normalise.UpstreamServer(c[0], c[1]) {
			t.Errorf("UpstreamServer(%q, %q) = true; the port differs and that is real",
				c[0], c[1])
		}
	}
}

func TestCIDR(t *testing.T) {
	t.Parallel()

	same := [][2]string{
		{"2001:db8:0:0::/32", "2001:db8::/32"},
		{"192.0.2.0/24", "192.0.2.0/24"},
	}
	differ := [][2]string{
		{"192.0.2.0/24", "192.0.2.0/25"},
		{"192.0.2.0/24", "198.51.100.0/24"},
	}

	for _, c := range same {
		if !normalise.CIDR(c[0], c[1]) {
			t.Errorf("CIDR(%q, %q) = false", c[0], c[1])
		}
	}
	for _, c := range differ {
		if normalise.CIDR(c[0], c[1]) {
			t.Errorf("CIDR(%q, %q) = true", c[0], c[1])
		}
	}
}

// TestTSIGKeyID covers the canonicalisation recorded from auth-5.1.3: a key
// created as "probe" gets the id "probe.".
func TestTSIGKeyID(t *testing.T) {
	t.Parallel()

	if !normalise.TSIGKeyID("probe", "probe.") {
		t.Error(`TSIGKeyID("probe", "probe.") = false; the server appends the dot`)
	}
	if normalise.TSIGKeyID("probe", "other.") {
		t.Error("two different key names compared equal")
	}
}

func TestStringMultiset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured []string
		actual     []string
		key        func(string) string
		want       bool
	}{
		{
			"order does not matter",
			[]string{"192.0.2.1", "192.0.2.2"},
			[]string{"192.0.2.2", "192.0.2.1"},
			normalise.IPAddressKey, true,
		},
		{
			"reordered and respelled",
			[]string{"2001:db8:0:0::1", "192.0.2.1"},
			[]string{"192.0.2.1", "2001:db8::1"},
			normalise.IPAddressKey, true,
		},
		{
			"a missing entry is a difference",
			[]string{"192.0.2.1", "192.0.2.2"},
			[]string{"192.0.2.1"},
			normalise.IPAddressKey, false,
		},
		{
			"an extra entry is a difference",
			[]string{"192.0.2.1"},
			[]string{"192.0.2.1", "192.0.2.2"},
			normalise.IPAddressKey, false,
		},
		{
			// Same length, one entry replaced: the guard against a comparison
			// that only checks counts.
			"a substituted entry is a difference",
			[]string{"192.0.2.1", "192.0.2.2"},
			[]string{"192.0.2.1", "192.0.2.3"},
			normalise.IPAddressKey, false,
		},
		{
			// A duplicate on one side must not match one entry twice.
			"a duplicate does not satisfy two slots",
			[]string{"192.0.2.1", "192.0.2.1"},
			[]string{"192.0.2.1", "192.0.2.2"},
			normalise.IPAddressKey, false,
		},
		{
			"empty lists are equal",
			nil, nil, normalise.IPAddressKey, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalise.StringMultiset(tt.configured, tt.actual, tt.key); got != tt.want {
				t.Errorf("StringMultiset(%v, %v) = %v, want %v",
					tt.configured, tt.actual, got, tt.want)
			}
		})
	}
}
