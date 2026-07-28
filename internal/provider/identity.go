package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// Resource identity, and the one property PowerDNS cannot give it.
//
// An identity is a separate object stored beside state, so Terraform can
// recognise a remote object without parsing an id string. The framework
// requires three things of it: it must address at most one remote object, it
// must let the provider decide whether that object exists, and it must not
// change for the lifetime of the object.
//
// This provider satisfies the second and third outright. Every identity here
// is the natural key PowerDNS itself uses, and every attribute that composes
// one already forces replacement — a zone cannot be renamed, an RRSet cannot
// change name or type, a TSIG key cannot change algorithm without being
// replaced. So an identity here is stable by construction rather than by
// promise.
//
// # The first property has a boundary worth stating
//
// "At most one remote object per provider, across all instances of that
// provider" is stricter than PowerDNS can support. `example.com.` names one
// zone on one server; two servers can each hold a zone by that name, and
// nothing in the API distinguishes them — `/servers/{id}` answers only for
// `localhost`, so there is no server identifier to compose in.
//
// Adding the endpoint URL would not fix it either, and would break the third
// property: a server that moves gets a new URL while remaining the same
// object, and the identity would change underneath it.
//
// So the boundary is the server, and `docs/contract.md` says so. Within one
// PowerDNS installation an identity is unique; across two, `example.com.` is
// ambiguous in exactly the way the zone name itself is. That is a property of
// the DNS rather than a shortcut taken here.

// zoneIdentitySchema is the identity of anything keyed by a zone name.
func zoneIdentitySchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"zone_name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description: "The canonical zone name. Unique within one PowerDNS " +
					"installation; see the provider contract on identity scope.",
			},
		},
	}
}

// IdentitySchema for powerdns_zone.
func (r *zoneResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = zoneIdentitySchema()
}

// IdentitySchema for powerdns_record.
//
// Three attributes rather than the composite id, because an identity is meant
// to be read by a program: splitting `example.com./www.example.com./A` on
// slashes is exactly the parsing an identity exists to avoid.
func (r *recordResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"zone_name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The zone holding the set.",
			},
			"record_name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The owner name.",
			},
			"record_type": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The record type.",
			},
		},
	}
}

// IdentitySchema for powerdns_zone_metadata.
func (r *zoneMetadataResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"zone_name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The zone.",
			},
			"kind": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The metadata kind.",
			},
		},
	}
}

// IdentitySchema for powerdns_zone_cryptokey.
//
// The key id is a number from a **global** counter rather than one per zone,
// so it is unique on its own — but the zone is part of the identity anyway,
// because every endpoint that addresses a key needs both and an identity that
// cannot be turned into a request is not much use.
func (r *cryptoKeyResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"zone_name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The zone the key signs.",
			},
			"key_id": identityschema.Int64Attribute{
				RequiredForImport: true,
				Description:       "The key's numeric id, from a server-global counter.",
			},
		},
	}
}

// IdentitySchema for powerdns_tsigkey.
func (r *tsigKeyResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"key_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description: "The canonical key name, which carries a trailing dot: " +
					"a key created as `transfer` has id `transfer.`.",
			},
		},
	}
}

// IdentitySchema for powerdns_view_zone.
func (r *viewZoneResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"view": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The view name.",
			},
			"zone_name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The zone placed in it.",
			},
		},
	}
}

// IdentitySchema for powerdns_network.
func (r *networkResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"network": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The subnet, in CIDR form.",
			},
		},
	}
}

// IdentitySchema for powerdns_autoprimary.
func (r *autoprimaryResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"ip": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The primary's address.",
			},
			"nameserver": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "The primary's nameserver name.",
			},
		},
	}
}

// IdentitySchema for powerdns_recursor_zone.
func (r *recursorZoneResource) IdentitySchema(
	_ context.Context,
	_ resource.IdentitySchemaRequest,
	resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = zoneIdentitySchema()
}

// The ACL resources deliberately have no identity.
//
// `powerdns_recursor_acl` and `powerdns_dnsdist_acl` are singletons on a
// server: their natural key is the setting name, which is `allow-from` on
// every installation there has ever been. An identity of `allow-from` would
// address one object per *server* and every object across servers, which is
// the property the framework asks for and the one thing this cannot supply.
//
// Leaving it out is honest. The resources import by setting name, as they
// already did, and nothing pretends the value distinguishes anything.

// setZoneIdentity writes the identity for a zone-keyed resource.
//
// Identity has to be set in Create *and* Read. Setting it only in Create
// leaves every resource that predates this change without one, and Terraform
// then has nothing to match on until the object is recreated.
func setZoneIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, zone string) diag.Diagnostics {
	return identity.SetAttribute(ctx, path.Root("zone_name"), zone)
}

// setRecordIdentity writes the identity for an RRSet.
func setRecordIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	zone, name, recordType string,
) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(identity.SetAttribute(ctx, path.Root("zone_name"), zone)...)
	diags.Append(identity.SetAttribute(ctx, path.Root("record_name"), name)...)
	diags.Append(identity.SetAttribute(ctx, path.Root("record_type"), recordType)...)
	return diags
}

// setMetadataIdentity writes the identity for a metadata kind.
func setMetadataIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	zone, kind string,
) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(identity.SetAttribute(ctx, path.Root("zone_name"), zone)...)
	diags.Append(identity.SetAttribute(ctx, path.Root("kind"), kind)...)
	return diags
}

// setCryptoKeyIdentity writes the identity for a DNSSEC key.
func setCryptoKeyIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	zone string,
	keyID int64,
) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(identity.SetAttribute(ctx, path.Root("zone_name"), zone)...)
	diags.Append(identity.SetAttribute(ctx, path.Root("key_id"), keyID)...)
	return diags
}

// setTSIGKeyIdentity writes the identity for a TSIG key.
func setTSIGKeyIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	keyID string,
) diag.Diagnostics {
	return identity.SetAttribute(ctx, path.Root("key_id"), keyID)
}

// setViewZoneIdentity writes the identity for a view membership.
func setViewZoneIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	view, zone string,
) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(identity.SetAttribute(ctx, path.Root("view"), view)...)
	diags.Append(identity.SetAttribute(ctx, path.Root("zone_name"), zone)...)
	return diags
}

// setNetworkIdentity writes the identity for a subnet mapping.
func setNetworkIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	network string,
) diag.Diagnostics {
	return identity.SetAttribute(ctx, path.Root("network"), network)
}

// setAutoprimaryIdentity writes the identity for an autoprimary.
func setAutoprimaryIdentity(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	ip, nameserver string,
) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(identity.SetAttribute(ctx, path.Root("ip"), ip)...)
	diags.Append(identity.SetAttribute(ctx, path.Root("nameserver"), nameserver)...)
	return diags
}
