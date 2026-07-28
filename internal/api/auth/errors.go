package auth

import "errors"

// ErrMissingChangeType is a client-side rejection, not a server one.
//
// PowerDNS answers a PATCH whose rrset carries no changetype with a bare 422
// whose message does not name the field. Catching it here turns a round trip
// and an opaque rejection into an immediate error naming the set.
var ErrMissingChangeType = errors.New(
	"every rrset in a patch needs changetype REPLACE or DELETE")
