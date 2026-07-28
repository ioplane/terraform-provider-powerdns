package rec

import (
	"context"
	"net/http"
)

// Zone is a Recursor zone.
//
// The Recursor's zone object is not the Authoritative one and shares only a
// name. There is no DNSSEC, no metadata, no SOA handling: a Recursor zone is
// either a forward instruction or a small authoritative island served locally.
type Zone struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// Kind is Native or Forwarded. Native means the Recursor answers from
	// Records; Forwarded means it asks Servers.
	Kind string `json:"kind,omitempty"`
	URL  string `json:"url,omitempty"`

	// Servers are the upstreams for a Forwarded zone, as host or host:port.
	Servers []string `json:"servers,omitempty"`
	// RecursionDesired sets the RD bit on forwarded queries. Forwarding to a
	// resolver wants it set; forwarding to an authoritative server does not,
	// and getting it wrong produces answers that look like a broken upstream.
	RecursionDesired *bool `json:"recursion_desired,omitempty"`
	// NotifyAllowed permits NOTIFY for this zone.
	NotifyAllowed *bool `json:"notify_allowed,omitempty"`

	// Records are served directly for a Native zone.
	Records []Record `json:"records,omitempty"`
}

// Record is one entry in a Recursor zone.
//
// Flat, unlike Authoritative's RRSet grouping: the Recursor's API has no
// rrsets and no changetype, so a record here is its own object.
type Record struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	TTL      uint32 `json:"ttl"`
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

// Zone kinds.
const (
	KindNative    = "Native"
	KindForwarded = "Forwarded"
)

// ListZones returns every zone, at GET /servers/localhost/zones.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var out []Zone
	err := c.http.Do(ctx, "list recursor zones", http.MethodGet,
		basePath()+"/zones", nil, &out)
	return out, err
}

// GetZone reads one zone, at GET /servers/localhost/zones/{id}.
func (c *Client) GetZone(ctx context.Context, zoneID string) (*Zone, error) {
	var out Zone
	if err := c.http.Do(ctx, "get recursor zone", http.MethodGet,
		zonePath(zoneID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateZone creates a zone, at POST /servers/localhost/zones.
//
// Needs webservice.api_dir; without it the answer is 422 naming the setting.
func (c *Client) CreateZone(ctx context.Context, zone Zone) (*Zone, error) {
	var out Zone
	if err := c.http.Do(ctx, "create recursor zone", http.MethodPost,
		basePath()+"/zones", zone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateZone replaces a zone, at PUT /servers/localhost/zones/{id}.
//
// A replace, not a merge: what is sent becomes the whole zone, so a caller
// that omits Records deletes them.
func (c *Client) UpdateZone(ctx context.Context, zoneID string, zone Zone) error {
	return c.http.Do(ctx, "update recursor zone", http.MethodPut,
		zonePath(zoneID), zone, nil)
}

// DeleteZone removes a zone, at DELETE /servers/localhost/zones/{id}.
func (c *Client) DeleteZone(ctx context.Context, zoneID string) error {
	return c.http.Do(ctx, "delete recursor zone", http.MethodDelete,
		zonePath(zoneID), nil, nil)
}
