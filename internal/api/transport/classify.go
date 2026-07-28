package transport

import (
	"net/http"
	"strings"
)

// Product identifies which PowerDNS server a client talks to. Classification
// needs it because the same status means different things across the three:
// a 405 from dnsdist is a capability limit, a 405 from the Authoritative server
// is a plain method error.
type Product int

const (
	ProductAuth Product = iota
	ProductRecursor
	ProductDNSDist
)

// String implements fmt.Stringer.
func (p Product) String() string {
	switch p {
	case ProductAuth:
		return "authoritative"
	case ProductRecursor:
		return "recursor"
	case ProductDNSDist:
		return "dnsdist"
	default:
		return "unknown"
	}
}

// classify decides whether a failure is an installation limit rather than an
// ordinary error.
//
// Every rule here was established by running the operation against a real
// server, not by reading documentation — none of these four conditions is
// documented in a way that would let a client infer them. The tag each was
// verified against is named on the rule.
func classify(product Product, method string, status int, path, serverMessage string) Capability {
	switch product {
	case ProductAuth:
		return classifyAuth(status, path)
	case ProductRecursor:
		return classifyRecursor(status, serverMessage)
	case ProductDNSDist:
		return classifyDNSDist(method, status, path)
	default:
		return CapabilityNone
	}
}

// classifyAuth: views and networks are unimplemented by the generic SQL
// backends. Verified against auth-5.1.3 on gpgsql and LMDB — the same POST
// answers 422 on the first and 204 on the second, and the gpgsql schema
// shipped in the image has no views or networks table.
//
// Only 422 is classified. A 404 on the same path means the view does not
// exist, and a 401 means the key is wrong; neither is a backend limit.
func classifyAuth(status int, path string) Capability {
	if status != http.StatusUnprocessableEntity {
		return CapabilityNone
	}
	if strings.Contains(path, "/views") || strings.Contains(path, "/networks") {
		return CapabilityViewsNeedLMDB
	}
	return CapabilityNone
}

// classifyRecursor: every write is refused unless api-config-dir is set. The
// Recursor names the setting in its own message, so the message is the signal
// rather than the path — that keeps an unrelated 422 from being mislabelled.
// Verified against rec-5.4.4.
func classifyRecursor(status int, serverMessage string) Capability {
	if status != http.StatusUnprocessableEntity {
		return CapabilityNone
	}
	if strings.Contains(serverMessage, "api-config-dir") {
		return CapabilityRecursorNeedsAPIDir
	}
	return CapabilityNone
}

// classifyDNSDist: two separate limits, and both look like something else.
//
// Only a 405 on a PUT is the setAPIWritable gate. isMethodAllowed() in
// dnsdist-web.cc (line 368 at tag dnsdist-2.1.0) admits GET unconditionally,
// PUT only when d_apiReadWrite is set and only for config/allow-from, and
// DELETE only for /api/v1/cache. Everything else falls through to false, so a
// POST answers 405 on a path that exists and has nothing to do with the flag.
// Both verified against dnsdist-2.1.0: POST config/allow-from answers 405 on a
// writable server, and the same PUT answers 405 with apiConfigDir alone and
// 200 once setAPIWritable(true, dir) is set.
//
// Classifying every 405 as the gate would tell an operator to enable
// setAPIWritable when the real problem is a method the product never accepts.
//
// A 404 on the cache path is a missing packet cache, not a missing endpoint.
// Verified the same way: 404 with no cache on the pool, 200 and
// {"count":"0","status":"purged"} after newPacketCache().
//
// DELETE /api/v1/cache is deliberately not gated: isMethodAllowed admits it
// without consulting d_apiReadWrite, so a cache flush works on a server that
// refuses every configuration write.
func classifyDNSDist(method string, status int, path string) Capability {
	switch status {
	case http.StatusMethodNotAllowed:
		if method == http.MethodPut {
			return CapabilityDNSDistNotWritable
		}
		return CapabilityNone
	case http.StatusNotFound:
		if strings.Contains(path, "/cache") {
			return CapabilityDNSDistNoPacketCache
		}
	}
	return CapabilityNone
}
