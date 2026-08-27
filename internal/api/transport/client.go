package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	// defaultTimeout bounds a single attempt, not the whole retried sequence.
	defaultTimeout = 30 * time.Second
	// defaultAttempts includes the first try, so 5 means one attempt and four
	// retries.
	defaultAttempts = 5
	// backoffBase and backoffCap give 1s, 2s, 4s, 8s.
	backoffBase = time.Second
	backoffCap  = 16 * time.Second
	// maxErrorBody bounds what is read from an error response. A server
	// misconfigured to return HTML should not be able to make the diagnostic
	// unreadable.
	maxErrorBody = 8 << 10
)

// Config describes one PowerDNS endpoint.
type Config struct {
	// BaseURL is the server root, without the /api/v1 suffix.
	BaseURL string
	// APIKey is sent as X-Api-Key, the canonical form Go writes on the wire.
	// All three products accept it; header names are case-insensitive.
	APIKey string
	// Product selects the capability rules applied to a failure.
	Product Product

	// CACertificate is a PEM bundle used to verify the server.
	CACertificate []byte
	// ClientCert is an optional client certificate for mutual TLS.
	ClientCert *tls.Certificate
	// InsecureSkipVerify disables verification. Behind a documented provider
	// argument only, never a default.
	InsecureSkipVerify bool

	// Timeout bounds one attempt. Zero means defaultTimeout.
	Timeout time.Duration
	// Attempts includes the first try. Zero means defaultAttempts.
	Attempts int
}

// Client issues requests to one PowerDNS server.
type Client struct {
	baseURL  string
	apiKey   string
	product  Product
	attempts int
	http     *http.Client
}

// New builds a client for one endpoint.
func New(cfg Config) (*Client, error) {
	tlsConfig := &tls.Config{
		// 1.2 rather than 1.3: the API is frequently published through a front
		// end that has not moved, and failing to reach the server is a worse
		// outcome than the difference between the two. Stated rather than
		// inherited, so a future toolchain default cannot lower it.
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // documented provider argument, never a default
	}

	if len(cfg.CACertificate) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(cfg.CACertificate) {
			return nil, fmt.Errorf("%w: no certificate found in the supplied bundle", ErrInvalidConfig)
		}
		tlsConfig.RootCAs = pool
	}

	if cfg.ClientCert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cfg.ClientCert}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	attempts := cfg.Attempts
	if attempts <= 0 {
		attempts = defaultAttempts
	}

	return &Client{
		baseURL:  cfg.BaseURL,
		apiKey:   cfg.APIKey,
		product:  cfg.Product,
		attempts: attempts,
		http: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}, nil
}

// ErrInvalidConfig is returned when a client cannot be built from the
// configuration given.
var ErrInvalidConfig = errors.New("invalid client configuration")

// Do issues a request and decodes a successful response into out.
//
// out may be nil for an operation that returns no body. The status is always
// examined before the body is touched: decoding an error response into the
// success type turns a 401 into an empty result, which is how a permissions
// problem comes to look like an empty zone list.
func (c *Client) Do(ctx context.Context, op, method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: encoding the request body: %w", op, err)
		}
	}

	var lastErr error
	for attempt := range c.attempts {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt); err != nil {
				return err
			}
			tflog.Debug(ctx, "retrying PowerDNS request", map[string]any{
				"op": op, "attempt": attempt + 1, "of": c.attempts,
			})
		}

		err := c.attempt(ctx, op, method, path, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err

		var apiErr *APIError
		if ok := asAPIError(err, &apiErr); ok && !apiErr.Retryable() {
			return err
		}
		if ctx.Err() != nil {
			return err
		}
	}

	return lastErr
}

// attempt performs one request.
func (c *Client) attempt(ctx context.Context, op, method, path string, payload []byte, out any) error {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("%s: building the request: %w", op, err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// A transport failure is retryable; the loop decides.
		return fmt.Errorf("%s: %s %s: %w", op, method, path, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			tflog.Warn(ctx, "closing the response body", map[string]any{
				"op": op, "error": cerr.Error(),
			})
		}
	}()

	// Status first. Always.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return c.apiError(op, method, path, resp)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: decoding the response: %w", op, err)
	}
	return nil
}

// apiError builds the typed error for a non-success response, including the
// capability classification.
func (c *Client) apiError(op, method, path string, resp *http.Response) error {
	message := readServerMessage(resp.Body)
	return &APIError{
		Op:            op,
		Method:        method,
		Path:          path,
		StatusCode:    resp.StatusCode,
		ServerMessage: message,
		Capability:    classify(c.product, method, resp.StatusCode, path, message),
	}
}

// readServerMessage extracts PowerDNS's own error text.
//
// All three products answer with {"error": "..."} on a failure, but a
// misconfigured front end may return HTML. Anything unparseable is returned as
// truncated text rather than discarded: the operator needs to see what actually
// came back.
func readServerMessage(body io.Reader) string {
	raw, err := io.ReadAll(io.LimitReader(body, maxErrorBody))
	if err != nil || len(raw) == 0 {
		return ""
	}

	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Error != "" {
		return envelope.Error
	}
	return string(bytes.TrimSpace(raw))
}

// sleepBackoff waits before retry attempt n (1-based), honouring cancellation.
func sleepBackoff(ctx context.Context, attempt int) error {
	delay := backoffDelay(attempt)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func backoffDelay(attempt int) time.Duration {
	delay := backoffBase
	for retry := 1; retry < attempt && delay < backoffCap; retry++ {
		delay = min(delay*2, backoffCap)
	}
	return delay
}
