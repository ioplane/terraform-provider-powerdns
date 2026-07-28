// Package transport carries the HTTP machinery shared by the three PowerDNS
// clients: request construction, authentication, retry, and the error type
// every client returns.
//
// Nothing outside internal/api builds an HTTP request. That rule exists because
// leaked request construction is what produces inconsistent status handling —
// some call sites checking the status before decoding the body and some not.
package transport

import (
	"errors"
	"fmt"
	"net/http"
)

// Capability classifies a failure that is a property of the installation
// rather than of the request.
//
// This is the type the whole design turns on. PowerDNS answers "this
// installation cannot do that" with a bare 4xx and a message about the
// immediate symptom, never about the requirement. Four such conditions exist
// across the three products; classifying them once here means every resource
// reports the same actionable diagnostic, and adding a resource cannot
// introduce a fifth interpretation of the same 422.
type Capability int

const (
	// CapabilityNone means the failure is an ordinary API error, not an
	// installation limit.
	CapabilityNone Capability = iota

	// CapabilityViewsNeedLMDB: views and networks are unimplemented by the
	// generic SQL backends. A read returns 200 with an empty collection while a
	// write returns 422, so only a write can distinguish "unsupported" from
	// "not configured".
	CapabilityViewsNeedLMDB

	// CapabilityRecursorNeedsAPIDir: the Recursor refuses every write unless
	// api-config-dir is set. It is unset by default, which makes the whole
	// Recursor API read-only out of the box.
	CapabilityRecursorNeedsAPIDir

	// CapabilityDNSDistNotWritable: dnsdist gates writes behind
	// setAPIWritable(). isMethodAllowed() checks that flag before it looks at
	// the path, so every PUT answers 405 without it — including paths that do
	// accept a PUT once it is enabled.
	CapabilityDNSDistNotWritable

	// CapabilityDNSDistNoPacketCache: DELETE /api/v1/cache answers 404 when the
	// pool has no packet cache. The 404 is about the pool, not the endpoint.
	CapabilityDNSDistNoPacketCache
)

// Requirement returns the operator-facing explanation of a capability limit:
// what is missing and where to change it. It is deliberately a sentence rather
// than a code, because it ends up in a Terraform diagnostic that someone reads
// while their apply is failing.
func (c Capability) Requirement() string {
	switch c {
	case CapabilityNone:
		return ""
	case CapabilityViewsNeedLMDB:
		return "Views and networks are only implemented by the LMDB backend. The " +
			"generic SQL backends have no tables for them, so a read returns an empty " +
			"list while a write fails like this. Check the launch= setting on the server."
	case CapabilityRecursorNeedsAPIDir:
		return "The Recursor only accepts writes when api-config-dir is set " +
			"(webservice.api_dir in the YAML settings). It is unset by default, which " +
			"makes the whole Recursor API read-only."
	case CapabilityDNSDistNotWritable:
		return "dnsdist only accepts writes when the API is made writable with " +
			"setAPIWritable(true, dir) in its Lua configuration. Setting apiConfigDir " +
			"in setWebserverConfig is not sufficient: the method check happens before " +
			"the path is examined, so every write answers 405 without it."
	case CapabilityDNSDistNoPacketCache:
		return "dnsdist reports 404 for a cache operation when the pool has no packet " +
			"cache. Attach one with newPacketCache() and getPool(\"\"):setCache(). The " +
			"404 is about the pool, not about the endpoint."
	default:
		return ""
	}
}

// String implements fmt.Stringer for logging and test output.
func (c Capability) String() string {
	switch c {
	case CapabilityNone:
		return "none"
	case CapabilityViewsNeedLMDB:
		return "views-need-lmdb"
	case CapabilityRecursorNeedsAPIDir:
		return "recursor-needs-api-dir"
	case CapabilityDNSDistNotWritable:
		return "dnsdist-not-writable"
	case CapabilityDNSDistNoPacketCache:
		return "dnsdist-no-packet-cache"
	default:
		return "unknown"
	}
}

// Sentinels for the conditions a caller branches on. Callers match with
// errors.Is; a typed *APIError carries the detail.
var (
	// ErrNotFound is a 404 that means the object does not exist.
	ErrNotFound = errors.New("not found")

	// ErrConflict is a 409: the object already exists, or the change collides.
	ErrConflict = errors.New("conflict")

	// ErrUnauthorized covers 401 and 403.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrRejected is a 422: the server understood the request and refused it.
	ErrRejected = errors.New("rejected")
)

// APIError is what every client method returns for a non-success response.
//
// It carries the status, the server's own message and the capability
// classification, so a resource can produce one diagnostic without
// re-interpreting the status itself.
type APIError struct {
	// Op is the logical operation, for example "create zone".
	Op string
	// Method and Path identify the request without exposing the full URL,
	// which may carry a host an operator would rather not see echoed.
	Method string
	Path   string
	// StatusCode is the HTTP status.
	StatusCode int
	// ServerMessage is the message PowerDNS returned, verbatim and unedited.
	ServerMessage string
	// Capability classifies an installation limit, or CapabilityNone.
	Capability Capability
}

// Error preserves the server's own wording and appends the requirement when
// one is known. The server message is never replaced: an operator searching for
// the text PowerDNS produced must find it.
func (e *APIError) Error() string {
	msg := fmt.Sprintf("%s: %s %s returned %d", e.Op, e.Method, e.Path, e.StatusCode)
	if e.ServerMessage != "" {
		msg += fmt.Sprintf(": %q", e.ServerMessage)
	}
	if req := e.Capability.Requirement(); req != "" {
		msg += "\n\n" + req
	}
	return msg
}

// Is lets callers use errors.Is with the sentinels above.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrConflict:
		return e.StatusCode == http.StatusConflict
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized ||
			e.StatusCode == http.StatusForbidden
	case ErrRejected:
		return e.StatusCode == http.StatusUnprocessableEntity
	default:
		return false
	}
}

// Retryable reports whether the request may be retried.
//
// Only 5xx and 429 qualify. A 4xx is an answer, not a flake: retrying a 404 or
// a 422 wastes the operator's time and can turn a fast failure into a slow one.
func (e *APIError) Retryable() bool {
	return e.StatusCode >= http.StatusInternalServerError ||
		e.StatusCode == http.StatusTooManyRequests
}
