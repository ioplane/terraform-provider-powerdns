package dnsdist

import (
	"context"
	"net/http"
	"net/url"
)

// ConfigSetting is one entry from the configuration dump.
//
// Value is `any` because it is not one type: `acl` comes back as a list of
// netmasks, most others as a string or a number.
type ConfigSetting struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// ACL is the `allow-from` setting, the only writable configuration there is.
type ACL struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Value []string `json:"value"`
}

// StatisticItem is one counter.
type StatisticItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// PoolStats is the state of one pool's downstream servers.
type PoolStats struct {
	Servers []DownstreamServer `json:"servers"`
}

// DownstreamServer is one backend behind dnsdist.
//
// Read-only, and not by omission: downstreams are declared in Lua or YAML and
// have no HTTP write path at all. See the package comment.
type DownstreamServer struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	State       string   `json:"state"`
	Pools       []string `json:"pools"`
	Qps         float64  `json:"qps"`
	Queries     uint64   `json:"queries"`
	Order       int      `json:"order"`
	Weight      int      `json:"weight"`
	Outstanding int      `json:"outstanding"`
	// HealthCheckFailures counts every failure since start, so a large number
	// on a healthy server means it was unreachable earlier, not now. State is
	// the field to read for current health.
	HealthCheckFailures uint64 `json:"healthCheckFailures"`
}

// Rings are the query and response ring buffers.
type Rings struct {
	Queries   []RingEntry `json:"queries"`
	Responses []RingEntry `json:"responses"`
}

// RingEntry is one recorded query or response.
type RingEntry struct {
	Query     string `json:"query"`
	QType     string `json:"qtype"`
	Requestor string `json:"requestor"`
	Size      int    `json:"size"`
	Age       int    `json:"age"`
	Protocol  string `json:"protocol"`
	// RCode and AnswerSize are only on responses.
	RCode      string `json:"rcode,omitempty"`
	AnswerSize int    `json:"answerSize,omitempty"`
}

// Server is dnsdist's summary object.
//
// Not a list, unlike the other two products: dnsdist answers
// GET /api/v1/servers/localhost with one object and registers no /servers
// collection at all. A client that probes reachability the way it does for
// Authoritative and the Recursor would 404 here.
type Server struct {
	DaemonType string             `json:"daemon_type"`
	Version    string             `json:"version"`
	ACL        string             `json:"acl"`
	Servers    []DownstreamServer `json:"servers"`
	Frontends  []any              `json:"frontends"`
	Pools      []any              `json:"pools"`
	Rules      []any              `json:"rules"`
}

// CacheFlushResult reports what a flush did.
//
// Count is a **string**, not a number — dnsdist serialises it as
// `{"count": "0", "status": "purged"}`. Decoding it as an int fails.
type CacheFlushResult struct {
	Count  string `json:"count"`
	Status string `json:"status"`
}

// GetServer reads the summary object, at GET /api/v1/servers/localhost.
//
// This is the reachability probe for dnsdist. See the Server comment for why
// it is not the /servers collection the other two products use.
func (c *Client) GetServer(ctx context.Context) (*Server, error) {
	var out Server
	if err := c.http.Do(ctx, "get dnsdist server", http.MethodGet,
		basePath(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetStatistics returns the counters, at
// GET /api/v1/servers/localhost/statistics.
func (c *Client) GetStatistics(ctx context.Context) ([]StatisticItem, error) {
	var out []StatisticItem
	err := c.http.Do(ctx, "get dnsdist statistics", http.MethodGet,
		basePath()+"/statistics", nil, &out)
	return out, err
}

// GetPool returns a pool's downstream servers, at
// GET /api/v1/servers/localhost/pool?name=...
//
// The name parameter is required even for the default pool, which is the empty
// string: omitting it answers 400 with an empty body. A pool nobody configured
// answers 404, also with an empty body, so neither failure carries a message —
// the operation name in the error is all an operator gets. Both verified
// against dnsdist-2.1.0.
func (c *Client) GetPool(ctx context.Context, name string) (*PoolStats, error) {
	params := url.Values{}
	params.Set("name", name)

	var out PoolStats
	if err := c.http.Do(ctx, "get dnsdist pool", http.MethodGet,
		basePath()+"/pool?"+params.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRings returns the query and response ring buffers, at
// GET /api/v1/servers/localhost/rings.
func (c *Client) GetRings(ctx context.Context) (*Rings, error) {
	var out Rings
	if err := c.http.Do(ctx, "get dnsdist rings", http.MethodGet,
		basePath()+"/rings", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetConfig returns the configuration dump, at
// GET /api/v1/servers/localhost/config.
//
// A dump, not a management surface: only the ACL can be written, and only
// through SetACL.
//
// The names do not line up, which is worth knowing before writing a resource
// that reconciles them. The dump calls the ACL `acl`; the writable endpoint is
// `config/allow-from`. Matching the dump entry against the endpoint path finds
// nothing. Verified against dnsdist-2.1.0, whose dump holds fifteen settings
// and no entry named allow-from.
func (c *Client) GetConfig(ctx context.Context) ([]ConfigSetting, error) {
	var out []ConfigSetting
	err := c.http.Do(ctx, "get dnsdist config", http.MethodGet,
		basePath()+"/config", nil, &out)
	return out, err
}

// GetACL reads allow-from, at
// GET /api/v1/servers/localhost/config/allow-from.
func (c *Client) GetACL(ctx context.Context) (*ACL, error) {
	var out ACL
	if err := c.http.Do(ctx, "get dnsdist acl", http.MethodGet,
		basePath()+"/config/allow-from", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetACL writes allow-from, at
// PUT /api/v1/servers/localhost/config/allow-from.
//
// One of the two writes dnsdist has. Needs `setAPIWritable(true, dir)`;
// without it the answer is 405 and the transport says which Lua call to add.
func (c *Client) SetACL(ctx context.Context, netmasks []string) error {
	body := ACL{Name: "allow-from", Type: "ConfigSetting", Value: netmasks}
	return c.http.Do(ctx, "set dnsdist acl", http.MethodPut,
		basePath()+"/config/allow-from", body, nil)
}

// FlushCache purges entries from a pool's packet cache, at
// DELETE /api/v1/cache?pool=...&name=...&type=...
//
// The other write, and the only endpoint outside /servers/localhost. Not gated
// by setAPIWritable — isMethodAllowed admits DELETE without consulting the
// flag — but it answers 404 when the pool has no packet cache, which the
// transport classifies rather than passing through as a not-found.
//
// pool is the empty string for the default pool.
func (c *Client) FlushCache(ctx context.Context, pool, name, qtype string) (*CacheFlushResult, error) {
	params := url.Values{}
	params.Set("pool", pool)
	params.Set("name", name)
	params.Set("type", qtype)

	var out CacheFlushResult
	if err := c.http.Do(ctx, "flush dnsdist cache", http.MethodDelete,
		"/api/v1/cache?"+params.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetJSONStats returns the legacy statistics object, at GET /jsonstat.
//
// Registered above the API handlers in dnsdist-web.cc as legacy dispatch, and
// outside /api entirely. It is here because it is the only place some counters
// appear, not because it is part of the API.
func (c *Client) GetJSONStats(ctx context.Context, command string) (map[string]any, error) {
	params := url.Values{}
	params.Set("command", command)

	var out map[string]any
	err := c.http.Do(ctx, "get dnsdist jsonstat", http.MethodGet,
		"/jsonstat?"+params.Encode(), nil, &out)
	return out, err
}
