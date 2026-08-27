package normalise_test

import (
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
)

func TestCanonicalKeysAreIdempotentAndMatchTheirEquivalence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		values []string
		key    func(string) string
		same   func(string, string) bool
	}{
		{"zone kind", []string{"native", "NATIVE", "Native", "K"}, normalise.ZoneKindKey, strings.EqualFold},
		{"DNS name", []string{"Example.COM", "example.com.", "other.example.", "EXAMPLE.."}, normalise.DNSNameKey, legacyDNSName},
		{"IP address", []string{"2001:db8:0:0::1", "2001:db8::1", "192.0.2.1", "invalid"}, normalise.IPAddressKey, legacyIPAddress},
		{"upstream", []string{"192.0.2.53", "192.0.2.53:53", "192.0.2.53:5353", "example.test", "EXAMPLE.test:53"}, normalise.UpstreamServerKey, legacyUpstream},
		{"CIDR", []string{"192.0.2.4/24", "192.0.2.0/24", "192.0.3.0/24", "invalid"}, normalise.CIDRKey, legacyCIDR},
		{"TSIG key", []string{"transfer", "TRANSFER.", "other.", "TRANSFER.."}, normalise.TSIGKeyIDKey, legacyDNSName},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, value := range test.values {
				if once, twice := test.key(value), test.key(test.key(value)); once != twice {
					t.Errorf("key is not idempotent for %q: %q then %q", value, once, twice)
				}
			}
			for _, left := range test.values {
				for _, right := range test.values {
					if got, want := test.key(left) == test.key(right), test.same(left, right); got != want {
						t.Errorf("key equality for %q/%q = %v, comparator = %v", left, right, got, want)
					}
				}
			}
		})
	}
}

func TestCanonicalMalformedUpstreamsFailClosed(t *testing.T) {
	t.Parallel()

	for _, values := range [][2]string{
		{"EXAMPLE..", "example.."},
		{"[", "["},
		{"host:broken:port", "HOST:broken:port"},
		{"host:", "host"},
		{"", ":53"},
	} {
		left, right := values[0], values[1]
		if normalise.UpstreamServerKey(left) == normalise.UpstreamServerKey(right) && left != right {
			t.Errorf("malformed upstreams %q/%q were treated as equivalent", left, right)
		}
		for _, value := range values {
			if once, twice := normalise.UpstreamServerKey(value), normalise.UpstreamServerKey(normalise.UpstreamServerKey(value)); once != twice {
				t.Errorf("malformed upstream key is not idempotent for %q: %q then %q", value, once, twice)
			}
		}
	}

	for _, value := range []string{"", ":53", "host:"} {
		if got := normalise.UpstreamServerKey(value); got != value {
			t.Errorf("malformed upstream %q canonicalized to %q", value, got)
		}
	}
}

func TestStringMultisetTracksOrderAndMultiplicity(t *testing.T) {
	t.Parallel()

	left := []string{"2001:db8:0:0::1", "192.0.2.1", "192.0.2.1"}
	right := []string{"192.0.2.1", "2001:db8::1", "192.0.2.1"}
	if !normalise.StringMultiset(left, right, normalise.IPAddressKey) {
		t.Fatal("permutation with equivalent spellings did not match")
	}

	replacement := slices.Clone(right)
	replacement[2] = "192.0.2.2"
	if normalise.StringMultiset(left, replacement, normalise.IPAddressKey) {
		t.Fatal("one value satisfied two multiplicity slots")
	}
}

func TestStringMultisetLargePathTracksMultiplicity(t *testing.T) {
	t.Parallel()

	configured := make([]string, 16)
	for index := range configured {
		configured[index] = "192.0.2." + strconv.Itoa(index)
	}
	actual := slices.Clone(configured)
	slices.Reverse(actual)
	if !normalise.StringMultiset(configured, actual, normalise.IPAddressKey) {
		t.Fatal("large permutation did not compare equal")
	}
	actual[0] = actual[1]
	if normalise.StringMultiset(configured, actual, normalise.IPAddressKey) {
		t.Fatal("large comparison ignored changed multiplicity")
	}
}

func FuzzCanonicalKeysAreIdempotent(f *testing.F) {
	for _, value := range []string{
		"example.com.", "2001:db8::1", "192.0.2.4/24", "[2001:db8::1]:53", "[", "invalid..",
	} {
		f.Add(value)
	}
	keys := []func(string) string{
		normalise.ZoneKindKey,
		normalise.DNSNameKey,
		normalise.IPAddressKey,
		normalise.UpstreamServerKey,
		normalise.CIDRKey,
		normalise.TSIGKeyIDKey,
	}
	f.Fuzz(func(t *testing.T, value string) {
		for _, key := range keys {
			if once, twice := key(value), key(key(value)); once != twice {
				t.Fatalf("key is not idempotent for %q: %q then %q", value, once, twice)
			}
		}
	})
}

func FuzzZoneKindKeyMatchesEqualFold(f *testing.F) {
	for _, values := range [][2]string{
		{"native", "NATIVE"},
		{"k", "K"},
		{"s", "ſ"},
		{"Σ", "ς"},
		{"\xa8", "\xc5"},
		{"different", "values"},
	} {
		f.Add(values[0], values[1])
	}
	f.Fuzz(func(t *testing.T, left, right string) {
		got := normalise.ZoneKindKey(left) == normalise.ZoneKindKey(right)
		if want := strings.EqualFold(left, right); got != want {
			t.Fatalf("canonical equality for %q/%q = %v, EqualFold = %v", left, right, got, want)
		}
	})
}

func FuzzCanonicalKeyEquivalenceDoesNotBroadenLegacy(f *testing.F) {
	for _, values := range [][2]string{
		{"example.com.", "EXAMPLE.COM"},
		{"2001:db8:0:0::1", "2001:db8::1"},
		{"192.0.2.4/24", "192.0.2.0/24"},
		{"EXAMPLE..", "example.."},
		{"\xa8", "\xc5"},
	} {
		f.Add(values[0], values[1])
	}
	f.Fuzz(func(t *testing.T, left, right string) {
		for _, contract := range []struct {
			name string
			key  func(string) string
			same func(string, string) bool
		}{
			{"zone kind", normalise.ZoneKindKey, strings.EqualFold},
			{"DNS name", normalise.DNSNameKey, legacyDNSName},
			{"IP address", normalise.IPAddressKey, legacyIPAddress},
			{"upstream", normalise.UpstreamServerKey, legacyUpstream},
			{"CIDR", normalise.CIDRKey, legacyCIDR},
			{"TSIG key", normalise.TSIGKeyIDKey, legacyDNSName},
		} {
			if contract.key(left) == contract.key(right) && !contract.same(left, right) {
				t.Errorf("%s key broadened legacy equivalence for %q/%q", contract.name, left, right)
			}
		}
	})
}

func FuzzCanonicalKeyEqualityLaws(f *testing.F) {
	for _, values := range [][3]string{
		{"example.com", "EXAMPLE.COM.", "example.com."},
		{"2001:db8:0:0::1", "2001:db8::1", "2001:0db8::1"},
		{"192.0.2.4/24", "192.0.2.0/24", "192.0.2.99/24"},
		{"\xa8", "\xc5", "\xff"},
	} {
		f.Add(values[0], values[1], values[2])
	}
	keys := []func(string) string{
		normalise.ZoneKindKey,
		normalise.DNSNameKey,
		normalise.IPAddressKey,
		normalise.UpstreamServerKey,
		normalise.CIDRKey,
		normalise.TSIGKeyIDKey,
	}
	f.Fuzz(func(t *testing.T, first, second, third string) {
		for _, key := range keys {
			firstKey, secondKey, thirdKey := key(first), key(second), key(third)
			if firstKey != key(first) {
				t.Fatal("canonical equality is not reflexive")
			}
			if (firstKey == secondKey) != (secondKey == firstKey) {
				t.Fatal("canonical equality is not symmetric")
			}
			if firstKey == secondKey && secondKey == thirdKey && firstKey != thirdKey {
				t.Fatal("canonical equality is not transitive")
			}
		}
	})
}

func legacyDNSName(left, right string) bool {
	return strings.EqualFold(strings.TrimSuffix(left, "."), strings.TrimSuffix(right, "."))
}

func legacyIPAddress(left, right string) bool {
	leftAddress, leftError := netip.ParseAddr(left)
	rightAddress, rightError := netip.ParseAddr(right)
	if leftError != nil || rightError != nil {
		return left == right
	}
	return leftAddress == rightAddress
}

func legacyCIDR(left, right string) bool {
	leftPrefix, leftError := netip.ParsePrefix(left)
	rightPrefix, rightError := netip.ParsePrefix(right)
	if leftError != nil || rightError != nil {
		return left == right
	}
	return leftPrefix.Masked() == rightPrefix.Masked()
}

func legacyUpstream(left, right string) bool {
	return legacyUpstreamKey(left) == legacyUpstreamKey(right)
}

func legacyUpstreamKey(value string) string {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		host, port = value, "53"
	}
	if port == "" {
		port = "53"
	}
	if address, err := netip.ParseAddr(host); err == nil {
		host = address.String()
	} else {
		host = strings.ToLower(strings.TrimSuffix(host, "."))
	}
	return net.JoinHostPort(host, port)
}
