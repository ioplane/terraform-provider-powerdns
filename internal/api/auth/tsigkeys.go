package auth

import (
	"context"
	"net/http"
	"net/url"
)

// TSIGKey is a shared secret for authenticating transfers and updates.
//
// # Key material
//
// Key is populated by CreateTSIGKey and GetTSIGKey, and comes back **empty**
// from ListTSIGKeys — the field is present but blank in the collection.
// Verified on 5.1.3. The same asymmetry as CryptoKey, and the same
// consequence: reconcile against the list, not against a get.
type TSIGKey struct {
	// ID is the canonical name, so a key created as "probe" has id "probe."
	// with a trailing dot. Using the requested name as a key would 404 on the
	// next read.
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// Algorithm is an HMAC name such as hmac-sha256. PowerDNS accepts the
	// short form and echoes it unchanged.
	Algorithm string `json:"algorithm,omitempty"`
	// Key is the base64 secret. Empty in a list response; see the type
	// comment. Sending it on create imports a key rather than generating one.
	Key string `json:"key,omitempty"`
}

func tsigKeyPath(keyID string) string {
	return basePath() + "/tsigkeys/" + url.PathEscape(keyID)
}

// ListTSIGKeys returns every key with the secret blanked, at
// GET /servers/localhost/tsigkeys.
func (c *Client) ListTSIGKeys(ctx context.Context) ([]TSIGKey, error) {
	var out []TSIGKey
	err := c.http.Do(ctx, "list tsigkeys", http.MethodGet,
		basePath()+"/tsigkeys", nil, &out)
	return out, err
}

// GetTSIGKey reads one key **including its secret**, at
// GET /servers/localhost/tsigkeys/{id}.
//
// Prefer ListTSIGKeys unless the secret is the point. See the type comment.
func (c *Client) GetTSIGKey(ctx context.Context, keyID string) (*TSIGKey, error) {
	var out TSIGKey
	if err := c.http.Do(ctx, "get tsigkey", http.MethodGet,
		tsigKeyPath(keyID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateTSIGKey generates or imports a key, at
// POST /servers/localhost/tsigkeys.
//
// Leaving Key empty asks PowerDNS to generate one; setting it imports the
// secret given. Either way the response carries the secret, and the returned
// ID is canonicalised — use it, not the name that was sent.
func (c *Client) CreateTSIGKey(ctx context.Context, key TSIGKey) (*TSIGKey, error) {
	var out TSIGKey
	if err := c.http.Do(ctx, "create tsigkey", http.MethodPost,
		basePath()+"/tsigkeys", key, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTSIGKey changes a key's name, algorithm or secret, at
// PUT /servers/localhost/tsigkeys/{id}.
func (c *Client) UpdateTSIGKey(ctx context.Context, keyID string, key TSIGKey) (*TSIGKey, error) {
	var out TSIGKey
	if err := c.http.Do(ctx, "update tsigkey", http.MethodPut,
		tsigKeyPath(keyID), key, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTSIGKey removes a key, at
// DELETE /servers/localhost/tsigkeys/{id}.
func (c *Client) DeleteTSIGKey(ctx context.Context, keyID string) error {
	return c.http.Do(ctx, "delete tsigkey", http.MethodDelete,
		tsigKeyPath(keyID), nil, nil)
}
