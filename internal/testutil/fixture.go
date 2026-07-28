// Package testutil holds the test scaffolding shared by the clients: recorded
// HTTP fixtures, a mock server that replays them, and the OpenAPI cross-check.
//
// The point of a fixture is that a client can be tested against what PowerDNS
// actually sends, without a container. Hand-written JSON in a test drifts from
// reality silently; a recording does not, because re-recording it against a
// newer server is one command and the diff is the change.
package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Product names the server a fixture was recorded from. It is part of the path
// because the same route means different things across the three.
type Product string

const (
	ProductAuth    Product = "auth"
	ProductRec     Product = "rec"
	ProductDNSDist Product = "dnsdist"
)

// Fixture is one recorded request and response.
type Fixture struct {
	// Name identifies the fixture within its product, and is the file stem.
	Name string `json:"name"`
	// Description says what the fixture is for, in a sentence. A fixture whose
	// purpose is not obvious from its name is worse than no fixture.
	Description string `json:"description"`

	Method string `json:"method"`
	Path   string `json:"path"`
	// Query is the raw query string, without the leading '?'.
	Query string `json:"query,omitempty"`

	Status int             `json:"status"`
	Body   json.RawMessage `json:"body,omitempty"`

	// RecordedAgainst names the server version the response came from, so a
	// stale fixture is visible rather than merely old.
	RecordedAgainst string `json:"recorded_against"`
}

// Key identifies a fixture for lookup by the mock server.
func (f Fixture) Key() string {
	if f.Query != "" {
		return f.Method + " " + f.Path + "?" + f.Query
	}
	return f.Method + " " + f.Path
}

// fixtureDir is where fixtures live, relative to this package.
const fixtureDir = "testdata/fixtures"

// Save writes a fixture, creating the product directory if needed.
func Save(root string, product Product, f Fixture) error {
	dir := filepath.Join(root, fixtureDir, string(product))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating the fixture directory: %w", err)
	}

	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding fixture %q: %w", f.Name, err)
	}
	raw = append(raw, '\n')

	path := filepath.Join(dir, f.Name+".json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("writing fixture %q: %w", f.Name, err)
	}
	return nil
}

// Load reads every fixture recorded for a product, sorted by name so test
// output is stable.
func Load(root string, product Product) ([]Fixture, error) {
	dir := filepath.Join(root, fixtureDir, string(product))

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	fixtures := make([]Fixture, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		var f Fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", entry.Name(), err)
		}
		fixtures = append(fixtures, f)
	}

	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].Name < fixtures[j].Name })
	return fixtures, nil
}
