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
	"unicode"
	"unicode/utf8"
)

// ZoneKind compares two zone kinds case-insensitively.
//
// PowerDNS title-cases whatever it is given: `native`, `NATIVE` and `Native`
// all become `Native`. Verified against auth-5.1.3.
func ZoneKind(configured, actual string) bool {
	return ZoneKindKey(configured) == ZoneKindKey(actual)
}

// ZoneKindKey canonicalises a PowerDNS zone kind for set comparison.
func ZoneKindKey(value string) string { return foldKey(value) }

// DNSName compares two DNS names, ignoring case and a trailing dot.
//
// The API is inconsistent about the trailing dot depending on the object, and
// DNS names are case-insensitive by definition. Comparing them as strings
// makes `Example.COM` and `example.com.` look like a change.
func DNSName(configured, actual string) bool {
	return DNSNameKey(configured) == DNSNameKey(actual)
}

// DNSNameKey canonicalises case and the optional root-label dot. Inputs with
// multiple trailing dots are invalid names and remain distinct rather than
// being silently repaired.
func DNSNameKey(value string) string {
	if strings.HasSuffix(value, "..") {
		return foldKey(value)
	}
	return foldKey(strings.TrimSuffix(value, "."))
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
	return IPAddressKey(configured) == IPAddressKey(actual)
}

// IPAddressKey returns the canonical address spelling, preserving invalid
// values exactly so two different malformed inputs cannot compare equal.
func IPAddressKey(value string) string {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return value
	}
	return address.String()
}

// UpstreamServer compares a Recursor forwarding target, treating a bare
// address as equivalent to the same address with the default DNS port.
//
// The Recursor appends `:53` to an upstream given without a port: `192.0.2.53`
// reads back `192.0.2.53:53`. Verified against rec-5.4.4. An explicit port
// other than 53 is a real difference and is compared as one.
func UpstreamServer(configured, actual string) bool {
	return UpstreamServerKey(configured) == UpstreamServerKey(actual)
}

// UpstreamServerKey canonicalises an upstream address and default port.
func UpstreamServerKey(value string) string { return upstreamKey(value) }

// upstreamKey reduces an upstream to address and port, defaulting the port and
// canonicalising the address.
func upstreamKey(value string) string {
	if value == "" {
		return value
	}

	host, port, err := net.SplitHostPort(value)
	if err != nil {
		// No port, or an IPv6 address written without brackets. Try it as a
		// bare address first, because that is the common case. Preserve
		// malformed bracket or colon syntax so normalization cannot hide it.
		if _, addressError := netip.ParseAddr(value); addressError != nil &&
			strings.ContainsAny(value, "[]:") {
			return value
		}
		host, port = value, defaultDNSPort
	} else if host == "" || port == "" {
		return value
	}
	// More than one root-label dot is invalid and remains visible as drift.
	if strings.HasSuffix(host, "..") {
		return value
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
	return CIDRKey(configured) == CIDRKey(actual)
}

// CIDRKey returns the masked canonical prefix, preserving invalid values.
func CIDRKey(value string) string {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return value
	}
	return prefix.Masked().String()
}

// TSIGKeyID compares a TSIG key name against the canonical id PowerDNS
// assigns, which is the name with a trailing dot.
//
// A key created as `probe` gets the id `probe.`. Verified against auth-5.1.3.
func TSIGKeyID(configured, actual string) bool {
	return TSIGKeyIDKey(configured) == TSIGKeyIDKey(actual)
}

// TSIGKeyIDKey canonicalises the key name PowerDNS uses as its id.
func TSIGKeyIDKey(value string) string { return DNSNameKey(value) }

const smallMultisetLimit = 8

// StringMultiset reports whether two lists hold the same canonical values,
// regardless of order and with duplicate multiplicity preserved.
//
// PowerDNS does not promise to preserve the order of a list it stores, and for
// a set of masters or netmasks the order carries no meaning. Comparing them as
// ordered lists turns a reordering into a diff.
func StringMultiset(configured, actual []string, key func(string) string) bool {
	if len(configured) != len(actual) {
		return false
	}
	if len(configured) <= smallMultisetLimit {
		return smallStringMultiset(configured, actual, key)
	}
	return countedStringMultiset(configured, actual, key)
}

func smallStringMultiset(configured, actual []string, key func(string) string) bool {
	var canonicalActual [smallMultisetLimit]string
	var used [smallMultisetLimit]bool
	for index, value := range actual {
		canonicalActual[index] = key(value)
	}
	for _, value := range configured {
		canonical := key(value)
		matched := false
		for index := range actual {
			if used[index] || canonical != canonicalActual[index] {
				continue
			}
			used[index], matched = true, true
			break
		}
		if !matched {
			return false
		}
	}
	return true
}

func countedStringMultiset(configured, actual []string, key func(string) string) bool {
	counts := make(map[string]int, len(configured))
	for _, value := range configured {
		counts[key(value)]++
	}
	for _, value := range actual {
		canonical := key(value)
		if counts[canonical] == 0 {
			return false
		}
		counts[canonical]--
	}
	return true
}

// RecordContent compares two record values, using whatever equivalence the
// record type actually has.
//
// PowerDNS rewrites the content of address records: an AAAA given as
// `2001:0db8:0000:0000:0000:0000:0000:0001` reads back `2001:db8::1`. Verified
// against auth-5.1.3. For every other type the content is compared exactly —
// a TXT record's quoting and whitespace are significant, and treating two
// spellings as equal there would hide a real edit.
func RecordContent(recordType, configured, actual string) bool {
	return RecordContentKey(recordType, configured) == RecordContentKey(recordType, actual)
}

// RecordContentKey canonicalises only record types PowerDNS rewrites.
func RecordContentKey(recordType, value string) string {
	switch strings.ToUpper(recordType) {
	case "A", "AAAA":
		return IPAddressKey(value)
	case "CNAME", "NS", "PTR", "DNAME":
		// Target names: case-insensitive, and PowerDNS stores them
		// canonicalised with a trailing dot.
		return DNSNameKey(value)
	default:
		return value
	}
}

// RecordName compares two record names.
//
// PowerDNS lowercases the owner name it is given: an rrset created as
// `TxT.example.com.` reads back `txt.example.com.`. Verified against
// auth-5.1.3.
func RecordName(configured, actual string) bool {
	return DNSNameKey(configured) == DNSNameKey(actual)
}

func foldKey(value string) string {
	if !utf8.ValidString(value) {
		var valid strings.Builder
		valid.Grow(len(value))
		for _, character := range value {
			valid.WriteRune(character)
		}
		value = valid.String()
	}

	asciiUpper := false
	asciiOnly := true
	for index := range len(value) {
		character := value[index]
		if character >= utf8.RuneSelf {
			asciiOnly = false
			break
		}
		asciiUpper = asciiUpper || character >= 'A' && character <= 'Z'
	}
	if asciiOnly {
		if !asciiUpper {
			return value
		}
		return strings.ToLower(value)
	}

	changed := false
	for _, character := range value {
		if foldRune(character) != character {
			changed = true
			break
		}
	}
	if !changed {
		return value
	}
	var result strings.Builder
	result.Grow(len(value))
	for _, character := range value {
		result.WriteRune(foldRune(character))
	}
	return result.String()
}

func foldRune(character rune) rune {
	canonical := character
	lower := rune(0)
	for folded := unicode.SimpleFold(character); folded != character; folded = unicode.SimpleFold(folded) {
		canonical = min(canonical, folded)
		if unicode.IsLower(folded) && (lower == 0 || folded < lower) {
			lower = folded
		}
	}
	if unicode.IsLower(character) && (lower == 0 || character < lower) {
		lower = character
	}
	if lower != 0 {
		return lower
	}
	return canonical
}

// DNSSECKeyType compares two DNSSEC key types, treating `csk` as compatible
// with both `ksk` and `zsk`.
//
// PowerDNS does not store the key type. It stores the DNSKEY flags — 257 for a
// key-signing key, 256 for a zone-signing key — and derives `keytype` from
// them together with **how many keys the zone holds**. Measured against
// auth-5.1.3:
//
//	keytype requested   zone contents        keytype read back   flags   ds
//	ksk                 no other key         csk                 257     2
//	zsk                 no other key         csk                 256     2
//	ksk                 a zsk beside it      ksk                 257     2
//	zsk                 a ksk beside it      zsk                 256     0
//
// So `csk` is not a third kind of key. It is what PowerDNS calls whichever key
// is doing every job because it is the only one, and the same key is renamed —
// not replaced — the moment a second appears. Same id, same material.
//
// Comparing the string literally is a trap that only springs in production: a
// second resource adding a key flips the first one's type, and a
// RequiresReplace on that attribute would destroy and recreate the signing key
// of a live zone, losing the DS the parent publishes.
func DNSSECKeyType(configured, actual string) bool {
	left, right := strings.ToLower(configured), strings.ToLower(actual)
	if left == right {
		return true
	}
	// csk is the sole-key spelling of either.
	return left == "csk" || right == "csk"
}
