// Package normalise holds the semantic comparisons PowerDNS forces on us.
//
// PowerDNS rewrites much of what it is given. A zone created as `native` reads
// back `Native`; a Recursor upstream given as `192.0.2.53` reads back
// `192.0.2.53:53`; an IPv6 address written `2001:db8:0:0::1` reads back
// `2001:db8::1`. Each of those is the same value spelled differently, and each
// produces a permanent diff if compared as a string.
//
// The functions here answer "are these the same thing?" for one kind of value.
// The plan modifiers in the sibling package apply them; keeping the comparison
// separate means it can be tested against the recorded fixtures without a
// Terraform plan in the way.
//
// Every rule here corresponds to a normalisation observed on a live server and
// recorded in docs/plan.md. None is speculative: a comparison that is looser
// than the server's is worse than a diff, because it hides a real change.
package normalise

import (
	"net"
	"net/netip"
	"strings"
)

// ZoneKind compares two zone kinds case-insensitively.
//
// PowerDNS title-cases whatever it is given: `native`, `NATIVE` and `Native`
// all become `Native`. Verified against auth-5.1.3.
func ZoneKind(configured, actual string) bool {
	return strings.EqualFold(configured, actual)
}

// DNSName compares two DNS names, ignoring case and a trailing dot.
//
// The API is inconsistent about the trailing dot depending on the object, and
// DNS names are case-insensitive by definition. Comparing them as strings
// makes `Example.COM` and `example.com.` look like a change.
func DNSName(configured, actual string) bool {
	return strings.EqualFold(
		strings.TrimSuffix(configured, "."),
		strings.TrimSuffix(actual, "."),
	)
}

// IPAddress compares two addresses by value rather than by spelling.
//
// IPv6 has many spellings of one address — `2001:db8:0:0:0:0:0:1` and
// `2001:db8::1` — and PowerDNS returns the compressed form regardless of what
// it was given. Verified against auth-5.1.3 on a zone master list.
//
// Falls back to string equality when either side does not parse, so a
// hostname or a malformed entry is compared exactly rather than silently
// treated as equal.
func IPAddress(configured, actual string) bool {
	left, errLeft := netip.ParseAddr(configured)
	right, errRight := netip.ParseAddr(actual)
	if errLeft != nil || errRight != nil {
		return configured == actual
	}
	return left == right
}

// UpstreamServer compares a Recursor forwarding target, treating a bare
// address as equivalent to the same address with the default DNS port.
//
// The Recursor appends `:53` to an upstream given without a port: `192.0.2.53`
// reads back `192.0.2.53:53`. Verified against rec-5.4.4. An explicit port
// other than 53 is a real difference and is compared as one.
func UpstreamServer(configured, actual string) bool {
	return upstreamKey(configured) == upstreamKey(actual)
}

// upstreamKey reduces an upstream to address and port, defaulting the port and
// canonicalising the address.
func upstreamKey(value string) string {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		// No port, or an IPv6 address written without brackets. Try it as a
		// bare address first, because that is the common case.
		host, port = value, defaultDNSPort
	}
	if port == "" {
		port = defaultDNSPort
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		host = addr.String()
	} else {
		host = strings.ToLower(strings.TrimSuffix(host, "."))
	}
	return net.JoinHostPort(host, port)
}

const defaultDNSPort = "53"

// CIDR compares two subnets by value.
//
// The same compression applies as for a bare address, and a prefix may be
// written with or without a canonical host part.
func CIDR(configured, actual string) bool {
	left, errLeft := netip.ParsePrefix(configured)
	right, errRight := netip.ParsePrefix(actual)
	if errLeft != nil || errRight != nil {
		return configured == actual
	}
	return left.Masked() == right.Masked()
}

// TSIGKeyID compares a TSIG key name against the canonical id PowerDNS
// assigns, which is the name with a trailing dot.
//
// A key created as `probe` gets the id `probe.`. Verified against auth-5.1.3.
func TSIGKeyID(configured, actual string) bool {
	return DNSName(configured, actual)
}

// StringSet reports whether two lists hold the same values under cmp,
// regardless of order.
//
// PowerDNS does not promise to preserve the order of a list it stores, and for
// a set of masters or netmasks the order carries no meaning. Comparing them as
// ordered lists turns a reordering into a diff.
//
// Quadratic, deliberately: these lists are a handful of entries, and cmp is
// not an equivalence a map key can express.
func StringSet(configured, actual []string, cmp func(a, b string) bool) bool {
	if len(configured) != len(actual) {
		return false
	}

	used := make([]bool, len(actual))
	for _, want := range configured {
		var found bool
		for i, got := range actual {
			if used[i] || !cmp(want, got) {
				continue
			}
			used[i], found = true, true
			break
		}
		if !found {
			return false
		}
	}
	return true
}
