package transport

import "errors"

// asAPIError is a thin wrapper over errors.As, kept so the retry loop reads as
// a decision rather than as reflection plumbing.
func asAPIError(err error, target **APIError) bool {
	return errors.As(err, target)
}
