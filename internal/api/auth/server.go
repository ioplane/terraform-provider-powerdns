package auth

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Server is the daemon's self-description.
type Server struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	DaemonType string `json:"daemon_type"`
	Version    string `json:"version"`
	URL        string `json:"url"`
	ConfigURL  string `json:"config_url"`
	ZonesURL   string `json:"zones_url"`
	// AutoprimariesURL is sent by every server and is absent from the
	// published Server schema, which sets additionalProperties: false —
	// divergence 4 in docs/plan.md. Decoding it here rather than dropping it
	// keeps the client honest about what arrived.
	AutoprimariesURL string `json:"autoprimaries_url"`
}

// ConfigSetting is one entry from the configuration dump.
type ConfigSetting struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// StatisticItem is one counter or gauge.
//
// Value is a string even for numbers, and the type varies —
// StatisticItem, MapStatisticItem, RingStatisticItem — so a caller that needs
// a number parses it and handles failure.
type StatisticItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// SearchResult is one hit from search-data.
//
// The shape depends on ObjectType: a zone hit has no content or ttl, a record
// hit has both. That is why the fields are optional rather than the type being
// split — the server returns one heterogeneous list.
type SearchResult struct {
	ObjectType string `json:"object_type"`
	ZoneID     string `json:"zone_id"`
	Zone       string `json:"zone,omitempty"`
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	TTL        uint32 `json:"ttl,omitempty"`
	Content    string `json:"content,omitempty"`
	Disabled   bool   `json:"disabled,omitempty"`
}

// ListServers returns every server, at GET /servers.
//
// There is exactly one, always called localhost. The endpoint exists because
// the API was shaped as though a daemon could host several.
func (c *Client) ListServers(ctx context.Context) ([]Server, error) {
	var out []Server
	err := c.http.Do(ctx, "list servers", http.MethodGet, "/api/v1/servers", nil, &out)
	return out, err
}

// GetServer reads the server object, at GET /servers/localhost.
//
// This is the cheapest proof that the API is reachable and the key is
// accepted, which is what the provider uses it for.
func (c *Client) GetServer(ctx context.Context) (*Server, error) {
	var out Server
	if err := c.http.Do(ctx, "get server", http.MethodGet, basePath(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetConfig returns the whole configuration, at GET /servers/localhost/config.
//
// Whole, and only whole. The specification also documents
// GET /config/{config_setting_name}, which has no handler and answers 404 —
// divergence 1 in docs/plan.md, reported as PowerDNS/pdns#17807. There is
// deliberately no method for it here.
func (c *Client) GetConfig(ctx context.Context) ([]ConfigSetting, error) {
	var out []ConfigSetting
	err := c.http.Do(ctx, "get config", http.MethodGet, basePath()+"/config", nil, &out)
	return out, err
}

// GetStatistics returns the counters, at GET /servers/localhost/statistics.
func (c *Client) GetStatistics(ctx context.Context) ([]StatisticItem, error) {
	var out []StatisticItem
	err := c.http.Do(ctx, "get statistics", http.MethodGet,
		basePath()+"/statistics", nil, &out)
	return out, err
}

// Search queries zones, records and comments, at
// GET /servers/localhost/search-data.
//
// The query supports * and ? wildcards. max bounds the result count; PowerDNS
// applies its own ceiling regardless.
func (c *Client) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	if maxResults > 0 {
		params.Set("max", strconv.Itoa(maxResults))
	}

	var out []SearchResult
	err := c.http.Do(ctx, "search", http.MethodGet,
		basePath()+"/search-data?"+params.Encode(), nil, &out)
	return out, err
}

// FlushCacheResult reports what a flush did.
type FlushCacheResult struct {
	Count  int    `json:"count"`
	Result string `json:"result"`
}

// FlushCache drops a domain from the cache, at
// PUT /servers/localhost/cache/flush?domain=...
//
// Count is how many entries went, and zero is a normal answer rather than a
// failure: nothing cached means nothing to drop.
func (c *Client) FlushCache(ctx context.Context, domain string) (*FlushCacheResult, error) {
	params := url.Values{}
	params.Set("domain", domain)

	var out FlushCacheResult
	if err := c.http.Do(ctx, "flush cache", http.MethodPut,
		basePath()+"/cache/flush?"+params.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
