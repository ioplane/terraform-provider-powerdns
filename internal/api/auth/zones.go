package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Zone is the Authoritative zone object.
//
// The field set is taken from what the server sends, verified against the
// recorded fixtures rather than against the published specification — the
// specification is demonstrably incomplete, see internal/testutil/spec.go.
//
// Pointer fields are the ones the server assigns or normalises. A plain bool
// cannot distinguish "the operator asked for false" from "the operator said
// nothing", and PATCH semantics need that distinction: sending dnssec=false
// unasked would silently unsign a zone.
type Zone struct {
	// ID is the canonical name and the primary key. It is not separately
	// settable: PowerDNS derives it from Name.
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// Kind is Native, Master, Slave, Producer or Consumer. The server
	// title-cases whatever it is given, so "native" comes back "Native".
	Kind string `json:"kind,omitempty"`
	URL  string `json:"url,omitempty"`

	// Serial is the zone's own serial; EditedSerial is what SOA-EDIT would
	// produce for a query right now. They differ whenever SOAEdit is set, and
	// only Serial is meaningful to compare against a previous read.
	Serial         uint32 `json:"serial,omitempty"`
	EditedSerial   uint32 `json:"edited_serial,omitempty"`
	NotifiedSerial uint32 `json:"notified_serial,omitempty"`
	LastCheck      int64  `json:"last_check,omitempty"`

	Masters []string `json:"masters,omitempty"`
	Account string   `json:"account,omitempty"`
	// Catalog is the catalog zone this zone belongs to. Empty means none.
	Catalog string `json:"catalog,omitempty"`

	DNSSEC      *bool  `json:"dnssec,omitempty"`
	NSEC3Param  string `json:"nsec3param,omitempty"`
	NSEC3Narrow *bool  `json:"nsec3narrow,omitempty"`
	Presigned   *bool  `json:"presigned,omitempty"`
	SOAEdit     string `json:"soa_edit,omitempty"`
	// SOAEditAPI is assigned by the server when a zone is created without one:
	// a POST that omits it reads back "DEFAULT". Comparing it as a string
	// against an empty configuration produces a permanent diff.
	SOAEditAPI string `json:"soa_edit_api,omitempty"`
	APIRectify *bool  `json:"api_rectify,omitempty"`

	MasterTSIGKeyIDs []string `json:"master_tsig_key_ids,omitempty"`
	SlaveTSIGKeyIDs  []string `json:"slave_tsig_key_ids,omitempty"`

	// RRSets carries the zone's records on read, and the initial contents on
	// create. It is never used to update: that is what PatchRRSets is for.
	RRSets []RRSet `json:"rrsets,omitempty"`
}

// RRSet is every record sharing a name and a type.
//
// PowerDNS has no per-record identity. The unit of change is the whole set:
// adding one A record means sending every A record for that name, and omitting
// one deletes it. A client that models individual records has to reconstruct
// the set before every write, and gets it wrong the first time two resources
// touch the same name.
type RRSet struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  uint32 `json:"ttl,omitempty"`
	// ChangeType is REPLACE or DELETE, and is only meaningful in a PATCH.
	ChangeType string    `json:"changetype,omitempty"`
	Records    []Record  `json:"records,omitempty"`
	Comments   []Comment `json:"comments,omitempty"`
}

// Record is one entry in an RRSet.
type Record struct {
	Content string `json:"content"`
	// Disabled keeps a record in the zone without serving it. It survives a
	// round trip and must be preserved: dropping it silently re-enables a
	// record an operator deliberately turned off.
	Disabled bool `json:"disabled"`
}

// Comment is an annotation attached to an RRSet.
type Comment struct {
	Content string `json:"content"`
	Account string `json:"account"`
	// ModifiedAt is assigned by the server. Sending it is accepted and
	// ignored; comparing it drives a diff on every plan.
	ModifiedAt int64 `json:"modified_at,omitempty"`
}

// Change types for a PATCH.
const (
	ChangeTypeReplace = "REPLACE"
	ChangeTypeDelete  = "DELETE"
)

// Zone kinds.
const (
	KindNative   = "Native"
	KindMaster   = "Master"
	KindSlave    = "Slave"
	KindProducer = "Producer"
	KindConsumer = "Consumer"
)

// ListZones returns every zone. GET /servers/localhost/zones
//
// The zones it returns carry no RRSets: PowerDNS omits them from the list to
// keep the response bounded. Reading a zone's records needs GetZone.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var out []Zone
	err := c.http.Do(ctx, "list zones", http.MethodGet, basePath()+"/zones", nil, &out)
	return out, err
}

// SearchZoneByName returns the zones whose name matches exactly.
//
// GET /servers/localhost/zones?zone=<name>. This is a filter on the list
// endpoint rather than a separate operation, and it is the cheap way to answer
// "does this zone exist" without pulling every zone in the installation.
func (c *Client) SearchZoneByName(ctx context.Context, name string) ([]Zone, error) {
	var out []Zone
	path := basePath() + "/zones?zone=" + url.QueryEscape(name)
	err := c.http.Do(ctx, "search zone by name", http.MethodGet, path, nil, &out)
	return out, err
}

// CreateZone creates a zone. POST /servers/localhost/zones
//
// The returned zone is the server's version, not the one sent: kind is
// title-cased, soa_edit_api is assigned if it was omitted, and IPv6 masters
// come back in compressed form. Callers must use what is returned.
func (c *Client) CreateZone(ctx context.Context, zone Zone) (*Zone, error) {
	var out Zone
	if err := c.http.Do(ctx, "create zone", http.MethodPost,
		basePath()+"/zones", zone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetZone reads a zone with its RRSets, at
// GET /servers/localhost/zones/{id}.
func (c *Client) GetZone(ctx context.Context, zoneID string) (*Zone, error) {
	var out Zone
	if err := c.http.Do(ctx, "get zone", http.MethodGet,
		zonePath(zoneID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateZone changes a zone's own attributes. PUT /servers/localhost/zones/{id}
//
// This is metadata only — kind, masters, account, the SOA-EDIT settings. It
// does not touch records; PowerDNS ignores rrsets in the body, so passing them
// here silently does nothing. Use PatchRRSets.
//
// The server answers 204 with no body, so the caller must re-read to see the
// result of any normalisation.
func (c *Client) UpdateZone(ctx context.Context, zoneID string, zone Zone) error {
	zone.RRSets = nil
	return c.http.Do(ctx, "update zone", http.MethodPut, zonePath(zoneID), zone, nil)
}

// PatchRRSets applies RRSet changes. PATCH /servers/localhost/zones/{id}
//
// Every set needs a ChangeType. REPLACE writes the set as given — an empty
// Records slice with REPLACE deletes it just as DELETE does, which is a sharp
// edge worth knowing about. DELETE removes the set and ignores Records.
//
// The whole patch is one transaction: either every set applies or none does.
// That is why this takes a slice rather than being called in a loop.
//
// The changetype check here saves a round trip rather than improving the
// message: PowerDNS answers a set without one with 422 "Key 'changetype' not
// present or not a String", which already names the field. Verified on 5.1.3.
func (c *Client) PatchRRSets(ctx context.Context, zoneID string, sets []RRSet) error {
	if len(sets) == 0 {
		return nil
	}
	for i, s := range sets {
		if s.ChangeType == "" {
			return fmt.Errorf("patch rrsets: %s %s has no changetype; %w",
				s.Name, s.Type, ErrMissingChangeType)
		}
		if s.ChangeType != ChangeTypeReplace && s.ChangeType != ChangeTypeDelete {
			return fmt.Errorf("patch rrsets: rrset %d has changetype %q; %w",
				i, s.ChangeType, ErrMissingChangeType)
		}
	}

	body := struct {
		RRSets []RRSet `json:"rrsets"`
	}{RRSets: sets}
	return c.http.Do(ctx, "patch rrsets", http.MethodPatch, zonePath(zoneID), body, nil)
}

// DeleteZone removes a zone, at
// DELETE /servers/localhost/zones/{id}.
func (c *Client) DeleteZone(ctx context.Context, zoneID string) error {
	return c.http.Do(ctx, "delete zone", http.MethodDelete, zonePath(zoneID), nil, nil)
}

// NotifyZone sends a NOTIFY to the zone's slaves.
// PUT /servers/localhost/zones/{id}/notify
//
// Imperative, not declarative: it has no state to converge on, which is why it
// belongs behind a Terraform action rather than a resource.
//
// Answers 200 {"result": "Notification queued"} even for a Native zone with no
// slaves to notify — queued is not delivered, and this reports the former.
func (c *Client) NotifyZone(ctx context.Context, zoneID string) error {
	return c.http.Do(ctx, "notify zone", http.MethodPut, zonePath(zoneID)+"/notify", nil, nil)
}

// AXFRRetrieveZone triggers a transfer from the zone's master.
// PUT /servers/localhost/zones/{id}/axfr-retrieve
//
// Slave zones only. Any other kind answers 422 "Domain … is not a secondary
// domain (or has no primary defined)". Verified on 5.1.3.
func (c *Client) AXFRRetrieveZone(ctx context.Context, zoneID string) error {
	return c.http.Do(ctx, "axfr-retrieve zone", http.MethodPut,
		zonePath(zoneID)+"/axfr-retrieve", nil, nil)
}

// ExportZone returns the zone in AXFR presentation format.
// GET /servers/localhost/zones/{id}/export
//
// The response is JSON — `{"zone": "…"}` — with the whole zone file as one
// string, tabs and newlines included. The documentation describes this
// endpoint as returning the zone in "AXFR format", which reads like text/plain
// and is not: verified against 5.1.3, which answers `Content-Type:
// application/json`.
func (c *Client) ExportZone(ctx context.Context, zoneID string) (string, error) {
	var out struct {
		Zone string `json:"zone"`
	}
	err := c.http.Do(ctx, "export zone", http.MethodGet,
		zonePath(zoneID)+"/export", nil, &out)
	return out.Zone, err
}

// RectifyZone recomputes DNSSEC ordering and NSEC records.
// PUT /servers/localhost/zones/{id}/rectify
//
// Answers 200 {"result": "Rectified"} on success, including for an unsigned
// Native zone and for one with api_rectify already on — neither is refused,
// contrary to what the operation's name suggests. A Slave zone with no
// transferred contents answers 422 "No SOA known". Verified on 5.1.3.
func (c *Client) RectifyZone(ctx context.Context, zoneID string) error {
	return c.http.Do(ctx, "rectify zone", http.MethodPut,
		zonePath(zoneID)+"/rectify", nil, nil)
}
