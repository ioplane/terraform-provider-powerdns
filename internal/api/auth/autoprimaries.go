package auth

import (
	"context"
	"net/http"
	"net/url"
)

// Autoprimary is a server permitted to create zones here by sending a NOTIFY.
//
// It has no id of its own: the pair of IP and nameserver is the key, which is
// why the delete path carries both and why there is no update — changing
// either field means deleting one entry and creating another.
type Autoprimary struct {
	IP         string `json:"ip"`
	Nameserver string `json:"nameserver"`
	Account    string `json:"account,omitempty"`
}

// ListAutoprimaries returns every autoprimary, at
// GET /servers/localhost/autoprimaries.
func (c *Client) ListAutoprimaries(ctx context.Context) ([]Autoprimary, error) {
	var out []Autoprimary
	err := c.http.Do(ctx, "list autoprimaries", http.MethodGet,
		basePath()+"/autoprimaries", nil, &out)
	return out, err
}

// CreateAutoprimary adds one, at POST /servers/localhost/autoprimaries.
//
// Answers 201 with no body, so there is nothing to read back and no server
// normalisation to worry about.
func (c *Client) CreateAutoprimary(ctx context.Context, ap Autoprimary) error {
	return c.http.Do(ctx, "create autoprimary", http.MethodPost,
		basePath()+"/autoprimaries", ap, nil)
}

// DeleteAutoprimary removes one, at
// DELETE /servers/localhost/autoprimaries/{ip}/{nameserver}.
func (c *Client) DeleteAutoprimary(ctx context.Context, ip, nameserver string) error {
	path := basePath() + "/autoprimaries/" + url.PathEscape(ip) + "/" + url.PathEscape(nameserver)
	return c.http.Do(ctx, "delete autoprimary", http.MethodDelete, path, nil, nil)
}
