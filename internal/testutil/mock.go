package testutil

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// MockServer replays recorded fixtures over HTTP.
//
// It exists so a client test can run against real PowerDNS payloads without a
// container: the lab is needed to *record* a fixture, not to use one. That
// keeps the contract layer fast enough to run on every commit while still
// testing against what the server actually sends.
//
// It is deliberately strict. An unmatched request is a test failure, not a
// 404, because a client asking for a route nobody recorded is exactly the
// mistake this layer should catch.
type MockServer struct {
	*httptest.Server

	t        *testing.T
	fixtures map[string]Fixture

	mu       sync.Mutex
	requests []RecordedRequest
	// APIKey, when set, is required on every request.
	APIKey string
}

// RecordedRequest is what the mock saw, for assertions about how a client
// behaves rather than only about what it returns.
type RecordedRequest struct {
	Method string
	Path   string
	Query  string
	APIKey string
}

// NewMockServer starts a server replaying the given fixtures. It is closed
// automatically when the test ends.
func NewMockServer(t *testing.T, fixtures []Fixture) *MockServer {
	t.Helper()

	m := &MockServer{
		t:        t,
		fixtures: make(map[string]Fixture, len(fixtures)),
		APIKey:   "testkey",
	}
	for _, f := range fixtures {
		m.fixtures[f.Key()] = f
	}

	m.Server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.Close)
	return m
}

// Requests returns what the mock saw, in order.
func (m *MockServer) Requests() []RecordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]RecordedRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

// AssertCalled fails the test if no request matched the method and path.
func (m *MockServer) AssertCalled(method, path string) {
	m.t.Helper()

	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			return
		}
	}
	m.t.Errorf("expected a %s to %s; the mock saw %d request(s)", method, path, len(m.Requests()))
}

func (m *MockServer) serve(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("X-Api-Key")

	m.mu.Lock()
	m.requests = append(m.requests, RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		APIKey: key,
	})
	m.mu.Unlock()

	if m.APIKey != "" && key != m.APIKey {
		w.WriteHeader(http.StatusUnauthorized)
		m.writeBody(w, []byte(`{"error":"Unauthorized"}`))
		return
	}

	lookup := r.Method + " " + r.URL.Path
	if r.URL.RawQuery != "" {
		if f, ok := m.fixtures[lookup+"?"+r.URL.RawQuery]; ok {
			m.write(w, f)
			return
		}
	}

	f, ok := m.fixtures[lookup]
	if !ok {
		// Fail the test rather than answer 404. A 404 would be indistinguishable
		// from a genuine not-found fixture and would quietly pass.
		// The test failure carries the detail. The response body deliberately
		// does not echo the request: reflecting input into a response is a habit
		// worth not having, even in a test double.
		m.t.Errorf("mock server: no fixture for %s\nrecord one with: task fixtures:record", lookup)
		w.WriteHeader(http.StatusNotImplemented)
		m.writeBody(w, []byte(`{"error":"no fixture for this request"}`))
		return
	}

	m.write(w, f)
}

func (m *MockServer) write(w http.ResponseWriter, f Fixture) {
	if len(f.Body) > 0 {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(f.Status)
	if len(f.Body) > 0 {
		m.writeBody(w, f.Body)
	}
}

// writeBody writes a response body, failing the test if it cannot. A test
// double that silently drops a write produces a confusing failure elsewhere.
func (m *MockServer) writeBody(w http.ResponseWriter, body []byte) {
	if _, err := w.Write(body); err != nil {
		m.t.Errorf("mock server: writing the response: %v", err)
	}
}
