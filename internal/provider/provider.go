// Package provider implements the PowerDNS provider on
// terraform-plugin-framework.
//
// The provider configures up to three independent clients — Authoritative,
// Recursor and dnsdist. Each is a separate web server with its own endpoint and
// its own API key; a client is nil when its endpoint is not configured, and a
// resource that needs one says so with a diagnostic rather than dereferencing
// it.
package provider

import (
	"context"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Defaults applied in Configure. The framework has no schema-level default for
// provider arguments, so the precedence — configuration, then environment,
// then default — is written out explicitly in the resolve helpers below.
const (
	defaultTimeoutSeconds = 30
	defaultRetryAttempts  = 5
)

var _ provider.Provider = (*powerdnsProvider)(nil)

type powerdnsProvider struct {
	version string
}

// New returns the provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &powerdnsProvider{version: version}
	}
}

// providerModel mirrors the schema below.
type providerModel struct {
	ServerURL         types.String `tfsdk:"server_url"`
	APIKey            types.String `tfsdk:"api_key"`
	RecursorServerURL types.String `tfsdk:"recursor_server_url"`
	RecursorAPIKey    types.String `tfsdk:"recursor_api_key"`
	DNSDistServerURL  types.String `tfsdk:"dnsdist_server_url"`
	DNSDistAPIKey     types.String `tfsdk:"dnsdist_api_key"`
	CACertificate     types.String `tfsdk:"ca_certificate"`
	ClientCertFile    types.String `tfsdk:"client_cert_file"`
	ClientCertKeyFile types.String `tfsdk:"client_cert_key_file"`
	InsecureHTTPS     types.Bool   `tfsdk:"insecure_https"`
	TimeoutSeconds    types.Int64  `tfsdk:"timeout_seconds"`
	RetryAttempts     types.Int64  `tfsdk:"retry_attempts"`
}

func (p *powerdnsProvider) Metadata(
	_ context.Context,
	_ provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = "powerdns"
	resp.Version = p.version
}

func (p *powerdnsProvider) Schema(
	_ context.Context,
	_ provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages PowerDNS Authoritative Server, Recursor and dnsdist " +
			"over their HTTP APIs. Configure only the products you use.",
		Attributes: map[string]schema.Attribute{
			"server_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the PowerDNS Authoritative API, for example " +
					"`https://pdns.example.com:8081`. Also `PDNS_SERVER_URL`.",
			},
			"api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "API key for the Authoritative API. Also `PDNS_API_KEY`. " +
					"Used as the fallback for the other products when they have no key of " +
					"their own.",
			},
			"recursor_server_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the Recursor API. Also " +
					"`PDNS_RECURSOR_SERVER_URL`.",
			},
			"recursor_api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "API key for the Recursor API. Also " +
					"`PDNS_RECURSOR_API_KEY`. Falls back to `api_key`, though the Recursor " +
					"is a separate web server and normally has its own key.",
			},
			"dnsdist_server_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the dnsdist API. Also " +
					"`PDNS_DNSDIST_SERVER_URL`.",
			},
			"dnsdist_api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "API key for the dnsdist API. Also " +
					"`PDNS_DNSDIST_API_KEY`. Falls back to `api_key`.",
			},
			"ca_certificate": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Content or path of a root CA used to verify the server " +
					"certificate. Also `PDNS_CACERT`.",
			},
			"client_cert_file": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Client certificate path for mutual TLS. Also `PDNS_CLIENT_CERT_FILE`.",
			},
			"client_cert_key_file": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Client key path for mutual TLS. Also `PDNS_CLIENT_CERT_KEY_FILE`.",
			},
			"insecure_https": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Skip verification of the server's TLS certificate. Also " +
					"`PDNS_INSECURE_HTTPS`. Intended for a lab; leaving it on in production " +
					"defeats the point of TLS.",
			},
			"timeout_seconds": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Per-request timeout in seconds. Defaults to 30.",
			},
			"retry_attempts": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Attempts for a retryable failure — transport errors and " +
					"`5xx`. `4xx` never retries: it is an answer, not a flake. Defaults to 5.",
			},
		},
	}
}

func (p *powerdnsProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverURL := resolveString(data.ServerURL, "PDNS_SERVER_URL", "")
	recursorURL := resolveString(data.RecursorServerURL, "PDNS_RECURSOR_SERVER_URL", "")
	dnsdistURL := resolveString(data.DNSDistServerURL, "PDNS_DNSDIST_SERVER_URL", "")

	// At least one product must be configured. Failing here with the list is
	// more use than three nil clients and a later dereference.
	if serverURL == "" && recursorURL == "" && dnsdistURL == "" {
		resp.Diagnostics.AddError(
			"No PowerDNS endpoint configured",
			"Set at least one of server_url, recursor_server_url or dnsdist_server_url, "+
				"or the corresponding PDNS_SERVER_URL, PDNS_RECURSOR_SERVER_URL or "+
				"PDNS_DNSDIST_SERVER_URL environment variable.",
		)
		return
	}

	clients, diags := buildClients(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "PowerDNS provider configured", map[string]any{
		"authoritative": clients.Auth != nil,
		"recursor":      clients.Recursor != nil,
		"dnsdist":       clients.DNSDist != nil,
	})

	// The same bundle to all three. Each has its own field, and forgetting one
	// is a nil dereference at apply rather than a compile error — which is how
	// the ephemeral resources first failed.
	resp.ResourceData = clients
	resp.DataSourceData = clients
	resp.EphemeralResourceData = clients
	resp.ActionData = clients
}

// Resources returns the managed resources.
func (p *powerdnsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewZoneResource,
		NewRecordResource,
		NewZoneMetadataResource,
		NewCryptoKeyResource,
		NewTSIGKeyResource,
		NewViewZoneResource,
		NewNetworkResource,
		NewAutoprimaryResource,
		NewRecursorZoneResource,
		NewRecursorACLResource,
		NewDNSDistACLResource,
	}
}

// EphemeralResources returns the ephemeral resources.
//
// These are the only place the provider asks PowerDNS for key material. The
// managed resources cannot return it by construction; an ephemeral value is
// safe to fetch because Terraform discards it rather than persisting it.
func (p *powerdnsProvider) EphemeralResources(
	_ context.Context,
) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewCryptoKeyMaterialEphemeral,
		NewTSIGKeySecretEphemeral,
	}
}

// Actions returns the imperative operations.
//
// Notifying, transferring, rectifying and flushing are things done *to* a zone
// rather than stated about it. Before Terraform 1.14 there was nowhere to put
// them, and the capability map listed 24 operations as uncoverable for that
// reason; actions cover 19 of them.
func (p *powerdnsProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewNotifyZoneAction,
		NewAXFRRetrieveAction,
		NewRectifyZoneAction,
		NewFlushCacheAction,
	}
}

// Functions returns the provider functions.
//
// All five are pure and offline. They exist so the name arithmetic a DNS
// configuration needs — qualifying a name, building a reverse zone, deriving a
// PTR — is done once and correctly, rather than as a locals block copied
// between modules and subtly wrong for IPv6.
func (p *powerdnsProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{
		NewFQDNFunction,
		NewIsFQDNFunction,
		NewReverseZoneNameFunction,
		NewPTRNameFunction,
		NewSOASerialFunction,
	}
}

// DataSources returns the data sources.
func (p *powerdnsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewZoneDataSource,
		NewZonesDataSource,
		NewRecordDataSource,
		NewZoneMetadataDataSource,
		NewZoneExportDataSource,
	}
}

// resolveString applies the precedence a provider argument needs: an explicit
// configuration value, then the environment, then the default.
func resolveString(v types.String, envVar, fallback string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	if env, ok := os.LookupEnv(envVar); ok && env != "" {
		return env
	}
	return fallback
}

func resolveBool(v types.Bool, envVar string, fallback bool) bool {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool()
	}
	if env, ok := os.LookupEnv(envVar); ok && env != "" {
		if parsed, err := strconv.ParseBool(env); err == nil {
			return parsed
		}
	}
	return fallback
}

func resolveInt(v types.Int64, envVar string, fallback int) int {
	if !v.IsNull() && !v.IsUnknown() {
		return int(v.ValueInt64())
	}
	if env, ok := os.LookupEnv(envVar); ok && env != "" {
		if parsed, err := strconv.Atoi(env); err == nil {
			return parsed
		}
	}
	return fallback
}
