package provider

import (
	"strings"
	"testing"
)

// The functions are pure, so they are tested as functions rather than through
// a Terraform plan. Each case is either a documented example — those must keep
// working, they are what a user copies — or a boundary that is easy to get
// wrong.

func TestReverseZoneName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cidr string
		want string
	}{
		// The documented examples.
		{"192.0.2.0/24", "2.0.192.in-addr.arpa."},
		{"2001:db8::/32", "8.b.d.0.1.0.0.2.ip6.arpa."},

		// The other two IPv4 boundaries.
		{"10.0.0.0/8", "10.in-addr.arpa."},
		{"172.16.0.0/16", "16.172.in-addr.arpa."},

		// A prefix given with host bits set is masked first, so this is the
		// same zone as 192.0.2.0/24.
		{"192.0.2.55/24", "2.0.192.in-addr.arpa."},

		// /0 is the root of the reverse tree and has no labels of its own.
		{"0.0.0.0/0", "in-addr.arpa."},

		// A /48 and a /64, the two IPv6 delegations that actually occur.
		{"2001:db8:abcd::/48", "d.c.b.a.8.b.d.0.1.0.0.2.ip6.arpa."},
		{
			"2001:db8:abcd:1234::/64",
			"4.3.2.1.d.c.b.a.8.b.d.0.1.0.0.2.ip6.arpa.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			t.Parallel()

			got, err := reverseZoneName(tt.cidr)
			if err != nil {
				t.Fatalf("reverseZoneName(%q): %v", tt.cidr, err)
			}
			if got != tt.want {
				t.Errorf("reverseZoneName(%q) = %q, want %q", tt.cidr, got, tt.want)
			}
		})
	}
}

// TestReverseZoneName_OffBoundary is the case worth refusing.
//
// A /25 spans two /24 reverse zones and a /20 spans sixteen, so there is no
// single name to return. Returning the enclosing /24 would look right and
// silently put half the PTRs in the wrong zone; RFC 2317 delegation is a
// different shape entirely and not something a name function can invent.
func TestReverseZoneName_OffBoundary(t *testing.T) {
	t.Parallel()

	for _, cidr := range []string{"192.0.2.0/25", "10.0.0.0/12", "2001:db8::/33"} {
		t.Run(cidr, func(t *testing.T) {
			t.Parallel()

			_, err := reverseZoneName(cidr)
			if err == nil {
				t.Fatalf("reverseZoneName(%q) returned a zone; a prefix off the "+
					"boundary spans several and has no single name", cidr)
			}
			// The message has to say which boundary, or an operator cannot act
			// on it.
			if !strings.Contains(err.Error(), "boundary") {
				t.Errorf("the error does not explain the boundary rule: %v", err)
			}
		})
	}
}

func TestReverseZoneName_Rejects(t *testing.T) {
	t.Parallel()

	for _, cidr := range []string{"", "192.0.2.1", "not-a-prefix", "192.0.2.0/33"} {
		if _, err := reverseZoneName(cidr); err == nil {
			t.Errorf("reverseZoneName(%q) accepted a value that is not a prefix", cidr)
		}
	}
}

func TestPTRName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		address string
		want    string
	}{
		{"192.0.2.1", "1.2.0.192.in-addr.arpa."},
		{"10.0.0.255", "255.0.0.10.in-addr.arpa."},
		{
			"2001:db8::1",
			"1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
		},
		{
			// Every nibble distinct, so a transposition would show.
			"2001:db8:abcd:1234:5678:9abc:def0:1234",
			"4.3.2.1.0.f.e.d.c.b.a.9.8.7.6.5.4.3.2.1.d.c.b.a.8.b.d.0.1.0.0.2.ip6.arpa.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			t.Parallel()

			got, err := ptrName(tt.address)
			if err != nil {
				t.Fatalf("ptrName(%q): %v", tt.address, err)
			}
			if got != tt.want {
				t.Errorf("ptrName(%q) =\n  %q\nwant\n  %q", tt.address, got, tt.want)
			}
		})
	}
}

// TestPTRName_IPv6Length pins the label count. An IPv6 PTR name is 32 nibbles
// plus ip6.arpa, and a loop that dropped one would still produce something
// plausible.
func TestPTRName_IPv6Length(t *testing.T) {
	t.Parallel()

	got, err := ptrName("2001:db8::1")
	if err != nil {
		t.Fatalf("ptrName: %v", err)
	}

	labels := strings.Split(strings.TrimSuffix(got, "."), ".")
	// 32 nibbles + "ip6" + "arpa"
	if len(labels) != 34 {
		t.Errorf("got %d labels, want 34 (32 nibbles plus ip6.arpa): %q",
			len(labels), got)
	}
}

func TestPTRName_Rejects(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"", "192.0.2.0/24", "example.com", "999.0.2.1"} {
		if _, err := ptrName(address); err == nil {
			t.Errorf("ptrName(%q) accepted a value that is not an address", address)
		}
	}
}

func TestSOASerial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		date     string
		revision int64
		want     int64
	}{
		{"2026-07-29", 1, 2026072901},
		{"2026-07-29", 0, 2026072900},
		{"2026-07-29", 99, 2026072999},
		{"2000-01-01", 0, 2000010100},
	}

	for _, tt := range tests {
		got, err := soaSerial(tt.date, tt.revision)
		if err != nil {
			t.Fatalf("soaSerial(%q, %d): %v", tt.date, tt.revision, err)
		}
		if got != tt.want {
			t.Errorf("soaSerial(%q, %d) = %d, want %d",
				tt.date, tt.revision, got, tt.want)
		}
	}
}

// TestSOASerial_RevisionBounds covers the boundary the convention imposes.
//
// A hundredth change in one day does not fit two digits. Rolling into the next
// day's serial would produce a number that looks fine and sorts wrong, so it
// is refused instead.
func TestSOASerial_RevisionBounds(t *testing.T) {
	t.Parallel()

	for _, revision := range []int64{-1, 100, 1000} {
		_, err := soaSerial("2026-07-29", revision)
		if err == nil {
			t.Errorf("soaSerial accepted revision %d", revision)
			continue
		}
		if !strings.Contains(err.Error(), "0 to 99") {
			t.Errorf("the error does not state the range: %v", err)
		}
	}
}

func TestSOASerial_RejectsABadDate(t *testing.T) {
	t.Parallel()

	for _, date := range []string{"", "29-07-2026", "2026-13-01", "today"} {
		if _, err := soaSerial(date, 1); err == nil {
			t.Errorf("soaSerial accepted %q as a date", date)
		}
	}
}

// TestSerialsSortByDateThenRevision is the property the convention exists for:
// a later change always produces a larger serial, which is what makes a
// secondary accept the transfer.
func TestSerialsSortByDateThenRevision(t *testing.T) {
	t.Parallel()

	ordered := []struct {
		date     string
		revision int64
	}{
		{"2026-07-29", 0},
		{"2026-07-29", 1},
		{"2026-07-29", 99},
		{"2026-07-30", 0},
		{"2026-08-01", 0},
		{"2027-01-01", 0},
	}

	var previous int64
	for _, entry := range ordered {
		got, err := soaSerial(entry.date, entry.revision)
		if err != nil {
			t.Fatalf("soaSerial(%q, %d): %v", entry.date, entry.revision, err)
		}
		if got <= previous {
			t.Errorf("serial %d for %s revision %d does not exceed the previous %d",
				got, entry.date, entry.revision, previous)
		}
		previous = got
	}
}
