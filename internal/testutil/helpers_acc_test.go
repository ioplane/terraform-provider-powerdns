//go:build acceptance

package testutil_test

import "bytes"

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
func newBuffer() *bytes.Buffer           { return &bytes.Buffer{} }
