package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Views and networks are LMDB-only.
//
// The gpgsql, gmysql and other relational backends do not implement them: a
// write answers 422 and the transport classifies it as
// CapabilityViewsNeedLMDB, so the diagnostic names the backend requirement
// rather than repeating a bare status. See internal/api/transport/classify.go.
//
// The API here is deliberately asymmetric with the rest of Authoritative.
// There is no view object: a view is a name that exists because zones point at
// it, and it stops existing when the last one is removed. Nothing creates or
// deletes a view directly.

// viewsPath is the collection.
func viewsPath() string {
	return basePath() + "/views"
}

func viewPath(view string) string {
	return viewsPath() + "/" + url.PathEscape(view)
}

// ListViews returns the names of every view, at GET /servers/localhost/views.
//
// The response is an object wrapping a list — {"views": [...]} — not a bare
// array, so this unwraps it.
func (c *Client) ListViews(ctx context.Context) ([]string, error) {
	var out struct {
		Views []string `json:"views"`
	}
	err := c.http.Do(ctx, "list views", http.MethodGet, viewsPath(), nil, &out)
	return out.Views, err
}

// GetView returns the zones in a view, at GET /servers/localhost/views/{view}.
//
// Also an object wrapping a list — {"zones": [...]}. A view that holds no
// zones does not exist, so an empty result and a 404 mean the same thing.
func (c *Client) GetView(ctx context.Context, view string) ([]string, error) {
	var out struct {
		Zones []string `json:"zones"`
	}
	err := c.http.Do(ctx, "get view", http.MethodGet, viewPath(view), nil, &out)
	return out.Zones, err
}

// AddZoneToView puts a zone in a view, at
// POST /servers/localhost/views/{view}.
//
// Creates the view as a side effect if it did not exist. Answers 204.
func (c *Client) AddZoneToView(ctx context.Context, view, zoneID string) error {
	body := struct {
		Name string `json:"name"`
	}{Name: zoneID}
	return c.http.Do(ctx, "add zone to view", http.MethodPost, viewPath(view), body, nil)
}

// RemoveZoneFromView takes a zone out of a view, at
// DELETE /servers/localhost/views/{view}/{zone_id}.
//
// Removing the last zone removes the view. Answers 204.
func (c *Client) RemoveZoneFromView(ctx context.Context, view, zoneID string) error {
	path := viewPath(view) + "/" + url.PathEscape(zoneID)
	return c.http.Do(ctx, "remove zone from view", http.MethodDelete, path, nil, nil)
}

// Network maps a client subnet to a view.
type Network struct {
	// Network is the subnet in CIDR form.
	Network string `json:"network"`
	// View is the view name. Empty in a write means "unassign".
	View string `json:"view"`
}

// networkPath builds the path from a CIDR.
//
// The API spells the network as two path segments rather than one — the
// address and the prefix length are separated by a slash, so "192.0.2.0/24"
// becomes ".../networks/192.0.2.0/24". Escaping the CIDR as a single segment
// would encode the slash and 404.
func networkPath(cidr string) string {
	// Split on the final slash, so an IPv6 CIDR keeps its colons and only the
	// prefix length is separated.
	i := strings.LastIndex(cidr, "/")
	if i < 0 {
		return basePath() + "/networks/" + url.PathEscape(cidr)
	}
	return basePath() + "/networks/" +
		url.PathEscape(cidr[:i]) + "/" + url.PathEscape(cidr[i+1:])
}

// ListNetworks returns every subnet-to-view mapping, at
// GET /servers/localhost/networks.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	var out struct {
		Networks []Network `json:"networks"`
	}
	err := c.http.Do(ctx, "list networks", http.MethodGet,
		basePath()+"/networks", nil, &out)
	return out.Networks, err
}

// GetNetwork reads one mapping, at
// GET /servers/localhost/networks/{ip}/{prefixlen}.
func (c *Client) GetNetwork(ctx context.Context, cidr string) (*Network, error) {
	var out Network
	if err := c.http.Do(ctx, "get network", http.MethodGet,
		networkPath(cidr), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetNetwork assigns a subnet to a view, at
// PUT /servers/localhost/networks/{ip}/{prefixlen}.
//
// An empty view removes the subnet's assignment. Answers 204, so there is nothing to read
// back.
func (c *Client) SetNetwork(ctx context.Context, cidr, view string) error {
	body := struct {
		View string `json:"view"`
	}{View: view}
	return c.http.Do(ctx, "set network", http.MethodPut, networkPath(cidr), body, nil)
}
