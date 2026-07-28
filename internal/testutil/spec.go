package testutil

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// The specification is a cross-check, never a source of truth and never a
// source of code.
//
// PowerDNS publishes an OpenAPI document for the Authoritative server only, and
// it diverges from the implementation in both directions — it documents
// GET /config/{name}, which has no handler, and omits POST on
// cryptokeys/{id}, which does. That is our own finding, reported as
// PowerDNS/pdns#17807.
//
// So the specification cannot decide whether an endpoint exists. What it is
// good for is catching drift in the shapes we do use: if a recorded response
// stops matching the schema it matched last month, something changed and
// somebody should look. Divergences we already know about are listed below and
// reported rather than failed, so the check stays useful instead of
// permanently red.
//
// Recursor and dnsdist publish no specification at all. Their fixtures are
// checked for well-formedness and nothing more; that is a property of PowerDNS,
// not a gap in this package.

// specPath is the vendored Authoritative specification.
const specPath = "testdata/spec/auth-5.1.3-openapi.yaml"

// KnownDivergence records a place where the specification and the server
// disagree, so the cross-check reports it once rather than failing forever.
//
// A divergence is either about a route the specification does not describe
// (Method and Path set) or about a field the server sends that the schema
// omits (Field set).
type KnownDivergence struct {
	Method string
	Path   string
	Field  string
	// Body marks a divergence in the response body of a route the
	// specification does describe, as opposed to a route it omits entirely.
	// The two need different matching: a route gap is found before validation
	// runs, a body mismatch only after.
	Body   bool
	Reason string
}

// KnownDivergences is the list as verified against auth-5.1.3 and master
// a74d89a8. Removing an entry when PowerDNS fixes it is the point: the list
// shrinking is how progress on #17807 becomes visible here.
var KnownDivergences = []KnownDivergence{
	{
		Method: http.MethodGet,
		Path:   "/api/v1/servers/localhost/config/{config_setting_name}",
		Reason: "documented but not implemented; ws-auth.cc registers /config only, and the " +
			"server answers 404 (PowerDNS/pdns#17807)",
	},
	{
		Method: http.MethodPost,
		Path:   "/api/v1/servers/localhost/zones/{zone_id}/cryptokeys/{cryptokey_id}",
		Reason: "implemented but not documented; registered at ws-auth.cc:3361 and answers 400 " +
			"rather than 404 (PowerDNS/pdns#17807)",
	},
	{
		Method: http.MethodGet,
		Path:   "/api/docs",
		Reason: "serves the specification itself and is absent from it",
	},
	{
		Method: http.MethodGet,
		Path:   "/metrics",
		Reason: "Prometheus endpoint, registered as a web handler and absent from the specification",
	},
	{
		Method: http.MethodGet,
		Path:   "/api/v1/servers/localhost/zones/{zone_id}/export",
		Body:   true,
		Reason: "the schema says type: string; the server sends an object, " +
			"{\"zone\": \"<the zone file>\"}. A generated client would hand back the " +
			"envelope. Found by this cross-check on a recorded fixture",
	},
	{
		Method: http.MethodPut,
		Path:   "/api/v1/servers/localhost/zones/{zone_id}/rectify",
		Body:   true,
		Reason: "the schema says type: string; the server sends an object, " +
			"{\"result\": \"Rectified\"}. Found by this cross-check on a recorded fixture",
	},
	{
		Field: "autoprimaries_url",
		Reason: "the Server object sends it and the schema omits it, with " +
			"additionalProperties: false — so a generated client would reject a real " +
			"response. Found by this cross-check, not by reading the specification",
	},
}

// SpecChecker validates recorded responses against the Authoritative
// specification.
type SpecChecker struct {
	doc    *openapi3.T
	router routers.Router
}

// NewSpecChecker loads and structurally validates the vendored specification.
//
// A failure here means the vendored file is broken, which is a problem with
// this repository rather than with PowerDNS.
func NewSpecChecker(ctx context.Context, root string) (*SpecChecker, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	doc, err := loader.LoadFromFile(filepath.Join(root, specPath))
	if err != nil {
		return nil, fmt.Errorf("loading the specification: %w", err)
	}

	// Structural validation is attempted but not required, because the
	// published specification does not currently pass it. auth-5.1.3 indents
	// Record.modified_at at the level of `properties` rather than inside it, so
	// the Record schema carries a stray sibling key and is simultaneously
	// missing the property. Comment, three lines below, nests it correctly.
	//
	// Failing here would make the cross-check permanently red and therefore
	// ignored, which is the opposite of what it is for. The defect is recorded
	// as a known one and reported upstream; when it is fixed this becomes a
	// hard failure again.
	if invalid := doc.Validate(ctx); invalid != nil {
		if !isKnownSpecDefect(invalid) {
			return nil, fmt.Errorf("the vendored specification is not valid: %w", invalid)
		}
	}

	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("building the router: %w", err)
	}

	return &SpecChecker{doc: doc, router: router}, nil
}

// knownSpecDefects are structural problems in the published specification that
// this repository tolerates rather than fixes, because the file is vendored
// verbatim and correcting it here would hide the defect.
var knownSpecDefects = []string{
	// auth-5.1.3 and master a74d89a8: Record.modified_at is indented one level
	// too far out, making it a sibling of `properties` instead of a property.
	`schema "RRSet": extra sibling fields: [modified_at]`,
}

func isKnownSpecDefect(err error) bool {
	msg := err.Error()
	for _, known := range knownSpecDefects {
		if strings.Contains(msg, known) {
			return true
		}
	}
	return false
}

// CheckResult is what a cross-check produced for one fixture.
type CheckResult struct {
	Fixture string
	// Matched is true when the specification described the route at all.
	Matched bool
	// Divergence is non-nil when the mismatch is one we already know about.
	Divergence *KnownDivergence
	// Err is a schema mismatch worth a human looking at.
	Err error
}

// Check validates one recorded response against the specification.
//
// A route the specification does not describe is reported, not failed: the
// specification is incomplete by demonstration, and treating every gap as an
// error would make the check useless.
func (s *SpecChecker) Check(ctx context.Context, f Fixture) CheckResult {
	result := CheckResult{Fixture: f.Name}

	target := f.Path
	if f.Query != "" {
		target += "?" + f.Query
	}

	req, reqErr := http.NewRequestWithContext(ctx, f.Method, "http://localhost"+target, nil)
	if reqErr != nil {
		result.Err = fmt.Errorf("building the request: %w", reqErr)
		return result
	}

	route, pathParams, routeErr := s.router.FindRoute(req)
	if routeErr != nil {
		if d := s.knownDivergence(f.Method, f.Path); d != nil {
			result.Divergence = d
		}
		return result
	}
	result.Matched = true

	// Only success responses are checked. PowerDNS's error bodies are a single
	// {"error": "..."} envelope that the specification models loosely, and
	// asserting against that would produce noise rather than signal.
	if f.Status < http.StatusOK || f.Status >= http.StatusMultipleChoices {
		return result
	}

	input := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
	}

	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: input,
		Status:                 f.Status,
		Header:                 http.Header{"Content-Type": []string{"application/json"}},
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
		},
	}
	if len(f.Body) > 0 {
		responseInput.SetBodyBytes(f.Body)
	}

	if verr := openapi3filter.ValidateResponse(ctx, responseInput); verr != nil {
		if d := s.knownBodyDivergence(f.Method, f.Path); d != nil {
			result.Divergence = d
			return result
		}
		if d := s.knownFieldDivergence(verr); d != nil {
			result.Divergence = d
			return result
		}
		result.Err = verr
	}
	return result
}

// DocumentedOperations returns method-and-path pairs the specification
// describes, for reporting coverage of what we actually exercise.
func (s *SpecChecker) DocumentedOperations() []string {
	var out []string
	for path, item := range s.doc.Paths.Map() {
		for method := range item.Operations() {
			out = append(out, method+" "+path)
		}
	}
	return out
}

func (s *SpecChecker) knownDivergence(method, path string) *KnownDivergence {
	for i := range KnownDivergences {
		d := &KnownDivergences[i]
		if d.Body || d.Method != method {
			continue
		}
		// Compare on the template's fixed prefix, so a recorded path with real
		// identifiers still matches a documented template.
		prefix, _, _ := strings.Cut(d.Path, "{")
		if strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")) {
			return d
		}
	}
	return nil
}

// knownBodyDivergence matches a response whose whole shape the specification
// gets wrong, keyed on the route rather than on the validator's message.
func (s *SpecChecker) knownBodyDivergence(method, path string) *KnownDivergence {
	for i := range KnownDivergences {
		d := &KnownDivergences[i]
		if !d.Body || d.Method != method {
			continue
		}
		prefix, _, _ := strings.Cut(d.Path, "{")
		suffix := d.Path[strings.LastIndex(d.Path, "}")+1:]
		if strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")) &&
			strings.HasSuffix(path, suffix) {
			return d
		}
	}
	return nil
}

// knownFieldDivergence matches a schema mismatch caused by a field the server
// sends and the specification omits.
func (s *SpecChecker) knownFieldDivergence(err error) *KnownDivergence {
	msg := err.Error()
	for i := range KnownDivergences {
		d := &KnownDivergences[i]
		if d.Field != "" && strings.Contains(msg, d.Field) {
			return d
		}
	}
	return nil
}
