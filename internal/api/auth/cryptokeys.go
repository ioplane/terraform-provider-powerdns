package auth

import (
	"context"
	"net/http"
	"strconv"
)

// CryptoKey is a DNSSEC key.
//
// # Key material
//
// PrivateKey is populated by CreateCryptoKey and GetCryptoKey, and is empty in
// ListCryptoKeys — PowerDNS omits it from the collection and includes it in
// the single-key response. Verified on 5.1.3.
//
// That asymmetry is load-bearing. Anything that reads a key to reconcile state
// must use ListCryptoKeys, because GetCryptoKey pulls the private key into
// process memory and, if the caller is careless, into Terraform state. The
// golden rule in AGENTS.md is that no secret reaches state; the mechanism for
// honouring it is choosing the right read, and the resource layer is expected
// to prove it with a test that greps the state file (S4-07).
type CryptoKey struct {
	// ID is an integer, unique within the zone rather than globally.
	ID      int    `json:"id,omitempty"`
	KeyType string `json:"keytype,omitempty"`
	Active  bool   `json:"active"`
	// Published controls whether the DNSKEY is served. A key can be active
	// (signing) but unpublished, which is how a rollover stages a key.
	Published *bool  `json:"published,omitempty"`
	DNSKey    string `json:"dnskey,omitempty"`
	// DS is present for a KSK and absent for a ZSK: a ZSK has no delegation
	// signer, so an empty slice here is normal rather than a failed read.
	DS        []string `json:"ds,omitempty"`
	Algorithm string   `json:"algorithm,omitempty"`
	Bits      int      `json:"bits,omitempty"`

	// PrivateKey is key material. See the type comment before using it.
	PrivateKey string `json:"privatekey,omitempty"`
}

// Key types.
const (
	KeyTypeKSK = "ksk"
	KeyTypeZSK = "zsk"
	KeyTypeCSK = "csk"
)

func cryptoKeyPath(zoneID string, keyID int) string {
	return zonePath(zoneID) + "/cryptokeys/" + strconv.Itoa(keyID)
}

// ListCryptoKeys returns a zone's keys without their private material, at
// GET /servers/localhost/zones/{id}/cryptokeys.
//
// This is the read to reconcile against. See the CryptoKey comment.
func (c *Client) ListCryptoKeys(ctx context.Context, zoneID string) ([]CryptoKey, error) {
	var out []CryptoKey
	err := c.http.Do(ctx, "list cryptokeys", http.MethodGet,
		zonePath(zoneID)+"/cryptokeys", nil, &out)
	return out, err
}

// GetCryptoKey reads one key, **including its private material**, at
// GET /servers/localhost/zones/{id}/cryptokeys/{key_id}.
//
// Prefer ListCryptoKeys unless the private key is the point — an ephemeral
// resource handing it to another provider, say. See the CryptoKey comment.
func (c *Client) GetCryptoKey(ctx context.Context, zoneID string, keyID int) (*CryptoKey, error) {
	var out CryptoKey
	if err := c.http.Do(ctx, "get cryptokey", http.MethodGet,
		cryptoKeyPath(zoneID, keyID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateCryptoKey generates or imports a key, at
// POST /servers/localhost/zones/{id}/cryptokeys.
//
// The response carries the private key whether it was generated or supplied;
// there is no way to ask the server not to send it back.
func (c *Client) CreateCryptoKey(ctx context.Context, zoneID string, key CryptoKey) (*CryptoKey, error) {
	var out CryptoKey
	if err := c.http.Do(ctx, "create cryptokey", http.MethodPost,
		zonePath(zoneID)+"/cryptokeys", key, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetCryptoKeyState activates or deactivates a key, at
// PUT /servers/localhost/zones/{id}/cryptokeys/{key_id}.
//
// Only active and published are changeable; PowerDNS ignores everything else
// in the body. Answers 204, so there is nothing to read back.
func (c *Client) SetCryptoKeyState(ctx context.Context, zoneID string, keyID int, active bool, published *bool) error {
	body := struct {
		Active    bool  `json:"active"`
		Published *bool `json:"published,omitempty"`
	}{Active: active, Published: published}

	return c.http.Do(ctx, "set cryptokey state", http.MethodPut,
		cryptoKeyPath(zoneID, keyID), body, nil)
}

// DeleteCryptoKey removes a key, at
// DELETE /servers/localhost/zones/{id}/cryptokeys/{key_id}.
func (c *Client) DeleteCryptoKey(ctx context.Context, zoneID string, keyID int) error {
	return c.http.Do(ctx, "delete cryptokey", http.MethodDelete,
		cryptoKeyPath(zoneID, keyID), nil, nil)
}

// RectifyAfterKeyChange is POST on a single cryptokey, at
// POST /servers/localhost/zones/{id}/cryptokeys/{key_id}.
//
// This operation is registered in ws-auth.cc and absent from the published
// specification — defect 2 of PowerDNS/pdns#17807. It answers 400 rather than
// 404, which is how the gap was found: a route that does not exist would not
// reject a request for being malformed.
//
// It is exposed for completeness of the 42-operation surface. Its semantics
// are not documented anywhere, so nothing in this provider calls it, and
// nothing should until PowerDNS says what it does.
func (c *Client) RectifyAfterKeyChange(ctx context.Context, zoneID string, keyID int) error {
	return c.http.Do(ctx, "post cryptokey", http.MethodPost,
		cryptoKeyPath(zoneID, keyID), nil, nil)
}
