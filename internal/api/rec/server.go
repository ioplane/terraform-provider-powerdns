package rec

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// The two writable settings.
//
// `/config/{name}` is not a general handler. ws-recursor.cc registers exactly
// two names — allow-from and allow-notify-from — and every other name answers
// 404 on both read and write. Verified on 5.4.4, and visible in the source at
// tag rec-5.4.4 as four separate registerApiHandler calls rather than one
// parameterised route.
//
// That is why these are two named methods and not a Get(name)/Set(name) pair:
// a map-shaped API would invite callers to try max-cache-entries and get a 404
// that reads like a missing server.
const (
	SettingAllowFrom       = "allow-from"
	SettingAllowNotifyFrom = "allow-notify-from"
)

// ConfigSetting is one configuration entry.
//
// Value is a list for the two writable settings, which are netmask groups, and
// the whole-config dump uses the same shape.
type ConfigSetting struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Value []string `json:"value"`
}

// Server is the daemon's self-description.
type Server struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	DaemonType string `json:"daemon_type"`
	Version    string `json:"version"`
	URL        string `json:"url"`
	ConfigURL  string `json:"config_url"`
	ZonesURL   string `json:"zones_url"`
}

// StatisticItem is one counter.
type StatisticItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// SearchResult is one hit from search-data.
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

// ErrSettingNotWritable is returned before a request is sent, for a name the
// Recursor does not register. See the constants above.
var ErrSettingNotWritable = fmt.Errorf(
	"the Recursor exposes only %q and %q by name; every other setting answers 404",
	SettingAllowFrom, SettingAllowNotifyFrom)

func settingPath(name string) string {
	return basePath() + "/config/" + url.PathEscape(name)
}

func isWritableSetting(name string) bool {
	return name == SettingAllowFrom || name == SettingAllowNotifyFrom
}

// ListServers returns every server, at GET /servers.
func (c *Client) ListServers(ctx context.Context) ([]Server, error) {
	var out []Server
	err := c.http.Do(ctx, "list recursor servers", http.MethodGet,
		"/api/v1/servers", nil, &out)
	return out, err
}

// GetServer reads the server object, at GET /servers/localhost.
func (c *Client) GetServer(ctx context.Context) (*Server, error) {
	var out Server
	if err := c.http.Do(ctx, "get recursor server", http.MethodGet,
		basePath(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetConfig returns the whole configuration, at GET /servers/localhost/config.
//
// Hundreds of settings, and all of them readable only here: none except the
// two writable ones can be read by name.
func (c *Client) GetConfig(ctx context.Context) ([]ConfigSetting, error) {
	var out []ConfigSetting
	err := c.http.Do(ctx, "get recursor config", http.MethodGet,
		basePath()+"/config", nil, &out)
	return out, err
}

// GetSetting reads one of the two named settings, at
// GET /servers/localhost/config/{name}.
//
// Rejects any other name before sending, because the 404 the server would
// answer is indistinguishable from a missing server.
func (c *Client) GetSetting(ctx context.Context, name string) (*ConfigSetting, error) {
	if !isWritableSetting(name) {
		return nil, fmt.Errorf("get recursor setting %q: %w", name, ErrSettingNotWritable)
	}

	var out ConfigSetting
	if err := c.http.Do(ctx, "get recursor setting", http.MethodGet,
		settingPath(name), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetSetting writes one of the two named settings, at
// PUT /servers/localhost/config/{name}.
//
// Needs webservice.api_dir. Without it the answer is 422 naming the setting,
// which the transport turns into a diagnostic saying so.
func (c *Client) SetSetting(ctx context.Context, name string, values []string) error {
	if !isWritableSetting(name) {
		return fmt.Errorf("set recursor setting %q: %w", name, ErrSettingNotWritable)
	}

	body := ConfigSetting{Name: name, Type: "ConfigSetting", Value: values}
	return c.http.Do(ctx, "set recursor setting", http.MethodPut,
		settingPath(name), body, nil)
}

// GetStatistics returns the counters, at GET /servers/localhost/statistics.
func (c *Client) GetStatistics(ctx context.Context) ([]StatisticItem, error) {
	var out []StatisticItem
	err := c.http.Do(ctx, "get recursor statistics", http.MethodGet,
		basePath()+"/statistics", nil, &out)
	return out, err
}

// GetRPZStatistics returns response-policy-zone counters, at
// GET /servers/localhost/rpzstatistics.
func (c *Client) GetRPZStatistics(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.http.Do(ctx, "get recursor rpz statistics", http.MethodGet,
		basePath()+"/rpzstatistics", nil, &out)
	return out, err
}

// Search queries zones and records, at GET /servers/localhost/search-data.
func (c *Client) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	if maxResults > 0 {
		params.Set("max", strconv.Itoa(maxResults))
	}

	var out []SearchResult
	err := c.http.Do(ctx, "search recursor", http.MethodGet,
		basePath()+"/search-data?"+params.Encode(), nil, &out)
	return out, err
}

// FlushCacheResult reports what a flush did. Count zero is a normal answer:
// nothing cached means nothing to drop.
type FlushCacheResult struct {
	Count  int    `json:"count"`
	Result string `json:"result"`
}

// FlushCache drops a domain from the cache, at
// PUT /servers/localhost/cache/flush?domain=...
func (c *Client) FlushCache(ctx context.Context, domain string) (*FlushCacheResult, error) {
	params := url.Values{}
	params.Set("domain", domain)

	var out FlushCacheResult
	if err := c.http.Do(ctx, "flush recursor cache", http.MethodPut,
		basePath()+"/cache/flush?"+params.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
