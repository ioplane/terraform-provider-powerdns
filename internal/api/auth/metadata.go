package auth

import (
	"context"
	"net/http"
	"net/url"
)

// Metadata is one zone metadata entry.
//
// Every kind is multi-valued, even the ones that only ever hold one item:
// SOA-EDIT-API is a list of one. Modelling it as a string would work until the
// first kind that genuinely repeats, such as ALLOW-AXFR-FROM.
type Metadata struct {
	Kind string `json:"kind"`
	// Metadata is the value list. The field name is the server's, not a
	// choice: the JSON key really is "metadata" inside an object of the same
	// name.
	Metadata []string `json:"metadata"`
}

// metadataPath builds the path for one kind.
func metadataPath(zoneID, kind string) string {
	return zonePath(zoneID) + "/metadata/" + url.PathEscape(kind)
}

// ListMetadata returns every metadata entry for a zone, at
// GET /servers/localhost/zones/{id}/metadata.
//
// A freshly created zone is not empty here: PowerDNS assigns SOA-EDIT-API
// DEFAULT on creation, so the list holds one entry nobody asked for. A caller
// that treats the whole list as managed state will try to delete it.
func (c *Client) ListMetadata(ctx context.Context, zoneID string) ([]Metadata, error) {
	var out []Metadata
	err := c.http.Do(ctx, "list zone metadata", http.MethodGet,
		zonePath(zoneID)+"/metadata", nil, &out)
	return out, err
}

// GetMetadata reads one kind, at
// GET /servers/localhost/zones/{id}/metadata/{kind}.
//
// An unset kind is not a 404: PowerDNS answers 200 with an empty value list.
// Absence and emptiness are the same state here, which is why this returns a
// value rather than a found flag.
func (c *Client) GetMetadata(ctx context.Context, zoneID, kind string) (*Metadata, error) {
	var out Metadata
	if err := c.http.Do(ctx, "get zone metadata", http.MethodGet,
		metadataPath(zoneID, kind), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateMetadata adds values to a kind, at
// POST /servers/localhost/zones/{id}/metadata.
//
// This appends rather than replaces, which is why SetMetadata exists and is
// almost always what a resource wants.
func (c *Client) CreateMetadata(ctx context.Context, zoneID string, m Metadata) (*Metadata, error) {
	var out Metadata
	if err := c.http.Do(ctx, "create zone metadata", http.MethodPost,
		zonePath(zoneID)+"/metadata", m, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetMetadata replaces a kind's values, at
// PUT /servers/localhost/zones/{id}/metadata/{kind}.
//
// This is the declarative one: what is sent is what the kind holds afterwards.
func (c *Client) SetMetadata(ctx context.Context, zoneID string, m Metadata) (*Metadata, error) {
	var out Metadata
	if err := c.http.Do(ctx, "set zone metadata", http.MethodPut,
		metadataPath(zoneID, m.Kind), m, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMetadata removes a kind, at
// DELETE /servers/localhost/zones/{id}/metadata/{kind}.
func (c *Client) DeleteMetadata(ctx context.Context, zoneID, kind string) error {
	return c.http.Do(ctx, "delete zone metadata", http.MethodDelete,
		metadataPath(zoneID, kind), nil, nil)
}
