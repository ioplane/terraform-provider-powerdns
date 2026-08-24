package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo_EncodesRequestJSON(t *testing.T) {
	t.Parallel()

	gotBody := make(chan string, 1)
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll: %v", err)
		}
		gotBody <- string(raw)
		w.WriteHeader(http.StatusNoContent)
	}))
	body := struct {
		Name string `json:"name"`
		TTL  int    `json:"ttl"`
	}{Name: "example.com.", TTL: 300}
	if err := client.Do(context.Background(), "create zone", http.MethodPost, "/zones", body, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := <-gotBody; got != `{"name":"example.com.","ttl":300}` {
		t.Errorf("body = %q", got)
	}
}

func TestDo_PreservesJSONV1ResponseSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{"duplicate name keeps last", []byte(`{"name":"first","name":"last"}`), "last"},
		{"invalid UTF-8 is replaced", []byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'}, "\ufffd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(tt.body) }))
			var out struct {
				Name string `json:"name"`
			}
			if err := client.Do(context.Background(), "get zone", http.MethodGet, "/zone", nil, &out); err != nil {
				t.Fatalf("Do: %v", err)
			}
			if out.Name != tt.want {
				t.Errorf("Name = %q, want %q", out.Name, tt.want)
			}
		})
	}
}

func TestDo_NoContentDoesNotDecode(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	out := struct{ Name string }{Name: "unchanged"}
	if err := client.Do(context.Background(), "delete zone", http.MethodDelete, "/zone", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Name != "unchanged" {
		t.Errorf("Name = %q, want unchanged", out.Name)
	}
}

func TestDo_BoundsAnOversizedErrorBody(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", maxErrorBody+1024)))
	}))
	err := client.Do(context.Background(), "list zones", http.MethodGet, "/zones", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if len(apiErr.ServerMessage) != maxErrorBody {
		t.Errorf("message length = %d, want %d", len(apiErr.ServerMessage), maxErrorBody)
	}
}

func TestDo_DrainsIgnoredSuccessBodyForConnectionReuse(t *testing.T) {
	t.Parallel()
	var remoteAddresses []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		remoteAddresses = append(remoteAddresses, r.RemoteAddr)
		mu.Unlock()
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	t.Cleanup(srv.Close)
	client, err := New(Config{BaseURL: srv.URL, Attempts: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for attempt := range 2 {
		if err := client.Do(context.Background(), "probe", http.MethodGet, "/", nil, nil); err != nil {
			t.Fatalf("Do: %v", err)
		}
		if attempt == 0 {
			// Go 1.27 drains an unread HTTP/1 response asynchronously for at
			// most 50 ms. Starting the next request immediately would race the
			// drain instead of testing connection reuse after it completes.
			time.Sleep(100 * time.Millisecond)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(remoteAddresses) != 2 || remoteAddresses[0] != remoteAddresses[1] {
		t.Fatalf("connections = %v, want one reused connection", remoteAddresses)
	}
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := New(Config{
		BaseURL:  srv.URL,
		APIKey:   "testkey",
		Product:  ProductAuth,
		Timeout:  2 * time.Second,
		Attempts: 3,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// TestDo_StatusBeforeBody is the regression guard for the defect this
// transport exists to avoid: decoding an error response into the success type.
// The handler returns 401 with a body that would unmarshal cleanly into the
// output struct, so a client that decoded first would report success with an
// empty result.
func TestDo_StatusBeforeBody(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
	}))

	var out []struct{ Name string }
	err := client.Do(context.Background(), "list zones", http.MethodGet, "/zones", nil, &out)

	if err == nil {
		t.Fatal("a 401 must be an error, not an empty result")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
	if len(out) != 0 {
		t.Errorf("out was populated from an error response: %v", out)
	}
}

func TestDo_SentinelMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   error
	}{
		{http.StatusNotFound, ErrNotFound},
		{http.StatusConflict, ErrConflict},
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrUnauthorized},
		{http.StatusUnprocessableEntity, ErrRejected},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			t.Parallel()

			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":"nope"}`))
			}))

			err := client.Do(context.Background(), "op", http.MethodGet, "/x", nil, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("status %d: err = %v, want %v", tt.status, err, tt.want)
			}
		})
	}
}

// TestDo_RetriesOnlyWhatIsWorthRetrying is the other half of the retry
// contract. A 4xx is an answer: retrying it turns a fast failure into a slow
// one and hides the cause behind a timeout.
func TestDo_RetriesOnlyWhatIsWorthRetrying(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       int
		wantAttempts int32
	}{
		{"500 retries", http.StatusInternalServerError, 3},
		{"429 retries", http.StatusTooManyRequests, 3},
		{"404 does not", http.StatusNotFound, 1},
		{"422 does not", http.StatusUnprocessableEntity, 1},
		{"401 does not", http.StatusUnauthorized, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tt.status)
			}))

			// A short deadline keeps the backoff from dominating the test; the
			// assertion is on the call count, which is reached well before it.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_ = client.Do(ctx, "op", http.MethodGet, "/x", nil, nil)

			if got := calls.Load(); got != tt.wantAttempts {
				t.Errorf("handler called %d times, want %d", got, tt.wantAttempts)
			}
		})
	}
}

func TestDo_SucceedsAfterATransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"example.com."}`))
	}))

	var out struct {
		Name string `json:"name"`
	}
	if err := client.Do(context.Background(), "get zone", http.MethodGet, "/z", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Name != "example.com." {
		t.Errorf("out.Name = %q, want example.com.", out.Name)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("handler called %d times, want 2", got)
	}
}

// TestDo_CarriesTheRequirement checks the end-to-end path: a 422 from a view
// write becomes an error whose text names the backend requirement, without
// discarding what PowerDNS said.
func TestDo_CarriesTheRequirement(t *testing.T) {
	t.Parallel()

	const serverText = "Failed to add example.com. to view trusted"

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"` + serverText + `"}`))
	}))

	err := client.Do(context.Background(), "add zone to view",
		http.MethodPost, "/api/v1/servers/localhost/views/trusted", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err is %T, want *APIError", err)
	}
	if apiErr.Capability != CapabilityViewsNeedLMDB {
		t.Errorf("Capability = %v, want %v", apiErr.Capability, CapabilityViewsNeedLMDB)
	}
	if !strings.Contains(err.Error(), serverText) {
		t.Error("the server's own message must be preserved")
	}
	if !strings.Contains(err.Error(), "LMDB") {
		t.Error("the requirement must be stated")
	}
}

func TestDo_SendsTheAPIKey(t *testing.T) {
	t.Parallel()

	var seen string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusNoContent)
	}))

	if err := client.Do(context.Background(), "op", http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if seen != "testkey" {
		t.Errorf("X-API-Key = %q, want testkey", seen)
	}
}

// TestDo_NonJSONErrorBody covers a front end returning HTML: the operator must
// still see what came back rather than an empty message.
func TestDo_NonJSONErrorBody(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<html><body>400 Bad Request</body></html>"))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Do(ctx, "op", http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("the body must survive into the error, got: %v", err)
	}
}

func TestDo_HonoursCancellation(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := client.Do(ctx, "op", http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	// The first backoff alone is a second; cancellation must cut it short.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("cancellation ignored: took %v", elapsed)
	}
}
