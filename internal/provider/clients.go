package provider

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/dnsdist"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/rec"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
)

// Clients is what every resource and data source receives.
//
// A field is nil when its product was not configured, which is the normal case
// — most installations run one of the three. Resources call the accessor for
// the product they need, and get a diagnostic naming the missing argument
// rather than a nil dereference.
type Clients struct {
	Auth     *auth.Client
	Recursor *rec.Client
	DNSDist  *dnsdist.Client
}

// clientConfig is the resolved configuration for one product.
type clientConfig struct {
	product transport.Product
	// name is what appears in a diagnostic: the argument an operator would set.
	name      string
	baseURL   string
	apiKey    string
	keyEnvVar string
}

// RequireAuth returns the Authoritative client or a diagnostic saying which
// argument to set.
//
// Every resource that needs a product calls its accessor first. The diagnostic
// names the argument and the environment variable, because "client is nil" is
// not something an operator can act on.
func (c *Clients) RequireAuth(resourceType string) (*auth.Client, diag.Diagnostics) {
	var diags diag.Diagnostics
	if c.Auth == nil {
		diags.AddError(
			"PowerDNS Authoritative server not configured",
			fmt.Sprintf("%s manages an object on the Authoritative server. Set the "+
				"provider's server_url argument, or the PDNS_SERVER_URL environment "+
				"variable.", resourceType),
		)
	}
	return c.Auth, diags
}

// RequireRecursor returns the Recursor client or a diagnostic.
func (c *Clients) RequireRecursor(resourceType string) (*rec.Client, diag.Diagnostics) {
	var diags diag.Diagnostics
	if c.Recursor == nil {
		diags.AddError(
			"PowerDNS Recursor not configured",
			fmt.Sprintf("%s manages an object on the Recursor. Set the provider's "+
				"recursor_server_url argument, or the PDNS_RECURSOR_SERVER_URL "+
				"environment variable.", resourceType),
		)
	}
	return c.Recursor, diags
}

// RequireDNSDist returns the dnsdist client or a diagnostic.
func (c *Clients) RequireDNSDist(resourceType string) (*dnsdist.Client, diag.Diagnostics) {
	var diags diag.Diagnostics
	if c.DNSDist == nil {
		diags.AddError(
			"PowerDNS dnsdist not configured",
			fmt.Sprintf("%s manages an object on dnsdist. Set the provider's "+
				"dnsdist_server_url argument, or the PDNS_DNSDIST_SERVER_URL "+
				"environment variable.", resourceType),
		)
	}
	return c.DNSDist, diags
}

// buildTransport constructs one product's transport client.
//
// TLS material is shared across the three: an installation running its own CA
// runs it for all of them, and asking for the same certificate three times
// would be a configuration tax with no benefit.
func buildTransport(cfg clientConfig, shared transport.Config) (*transport.Client, diag.Diagnostics) {
	var diags diag.Diagnostics

	if cfg.apiKey == "" {
		diags.AddError(
			"No API key for "+cfg.name,
			fmt.Sprintf("%s is configured but no API key was given. Set the matching "+
				"api_key argument, or the %s environment variable. PowerDNS rejects "+
				"every request without one.", cfg.name, cfg.keyEnvVar),
		)
		return nil, diags
	}

	shared.BaseURL = cfg.baseURL
	shared.APIKey = cfg.apiKey
	shared.Product = cfg.product

	client, err := transport.New(shared)
	if err != nil {
		diags.AddError(
			"Cannot build the "+cfg.name+" client",
			err.Error(),
		)
		return nil, diags
	}
	return client, nil
}

// buildClients resolves the configuration into whichever clients were asked
// for.
func buildClients(data providerModel) (*Clients, diag.Diagnostics) {
	shared, diags := buildSharedConfig(data)
	if diags.HasError() {
		return nil, diags
	}

	clients := &Clients{}

	configs := []struct {
		cfg    clientConfig
		assign func(*transport.Client)
	}{
		{
			clientConfig{
				product:   transport.ProductAuth,
				name:      "server_url",
				baseURL:   resolveString(data.ServerURL, "PDNS_SERVER_URL", ""),
				apiKey:    resolveString(data.APIKey, "PDNS_API_KEY", ""),
				keyEnvVar: "PDNS_API_KEY",
			},
			func(c *transport.Client) { clients.Auth = auth.New(c) },
		},
		{
			clientConfig{
				product: transport.ProductRecursor,
				name:    "recursor_server_url",
				baseURL: resolveString(data.RecursorServerURL, "PDNS_RECURSOR_SERVER_URL", ""),
				// Falls back to the Authoritative key: one operator commonly
				// runs both with the same key, and demanding it twice is a
				// tax rather than a safeguard.
				apiKey: resolveString(data.RecursorAPIKey, "PDNS_RECURSOR_API_KEY",
					resolveString(data.APIKey, "PDNS_API_KEY", "")),
				keyEnvVar: "PDNS_RECURSOR_API_KEY",
			},
			func(c *transport.Client) { clients.Recursor = rec.New(c) },
		},
		{
			clientConfig{
				product: transport.ProductDNSDist,
				name:    "dnsdist_server_url",
				baseURL: resolveString(data.DNSDistServerURL, "PDNS_DNSDIST_SERVER_URL", ""),
				apiKey: resolveString(data.DNSDistAPIKey, "PDNS_DNSDIST_API_KEY",
					resolveString(data.APIKey, "PDNS_API_KEY", "")),
				keyEnvVar: "PDNS_DNSDIST_API_KEY",
			},
			func(c *transport.Client) { clients.DNSDist = dnsdist.New(c) },
		},
	}

	for _, entry := range configs {
		if entry.cfg.baseURL == "" {
			continue
		}
		client, d := buildTransport(entry.cfg, shared)
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		entry.assign(client)
	}

	return clients, diags
}

// buildSharedConfig resolves what every product's client has in common:
// timeouts, retries and TLS material.
//
// TLS is shared deliberately. An installation running its own CA runs it for
// all three products, and asking for the same certificate three times would be
// a configuration tax with no benefit.
func buildSharedConfig(data providerModel) (transport.Config, diag.Diagnostics) {
	var diags diag.Diagnostics

	shared := transport.Config{
		Timeout: time.Duration(resolveInt(
			data.TimeoutSeconds, "PDNS_TIMEOUT_SECONDS", defaultTimeoutSeconds)) * time.Second,
		Attempts: resolveInt(data.RetryAttempts, "PDNS_RETRY_ATTEMPTS", defaultRetryAttempts),
		InsecureSkipVerify: resolveBool(
			data.InsecureHTTPS, "PDNS_INSECURE_HTTPS", false),
	}

	if ca := resolveString(data.CACertificate, "PDNS_CA_CERTIFICATE", ""); ca != "" {
		pem, err := readIfFile(ca)
		if err != nil {
			diags.AddError("Cannot read the CA certificate", err.Error())
			return shared, diags
		}
		shared.CACertificate = pem
	}

	certFile := resolveString(data.ClientCertFile, "PDNS_CLIENT_CERT_FILE", "")
	keyFile := resolveString(data.ClientCertKeyFile, "PDNS_CLIENT_CERT_KEY_FILE", "")
	switch {
	case certFile != "" && keyFile == "", certFile == "" && keyFile != "":
		diags.AddError(
			"Incomplete client certificate",
			"client_cert_file and client_cert_key_file must be set together; a "+
				"certificate without its key cannot be presented.",
		)
	case certFile != "":
		pair, err := loadClientCertificate(certFile, keyFile)
		if err != nil {
			diags.AddError("Cannot load the client certificate", err.Error())
			return shared, diags
		}
		shared.ClientCert = pair
	}

	return shared, diags
}

// loadClientCertificate reads a certificate and key pair from disk.
func loadClientCertificate(certFile, keyFile string) (*tls.Certificate, error) {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("loading %s and %s: %w", certFile, keyFile, err)
	}
	return &pair, nil
}

// readIfFile accepts either PEM text or a path to a file holding it.
//
// Both spellings appear in the wild — a heredoc in HCL, or a path from a
// secret manager — and telling them apart by looking for the PEM header is
// less surprising than making the operator say which they meant.
func readIfFile(value string) ([]byte, error) {
	if len(value) > 0 && value[0] == '-' {
		return []byte(value), nil
	}
	raw, err := os.ReadFile(value)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", value, err)
	}
	return raw, nil
}
