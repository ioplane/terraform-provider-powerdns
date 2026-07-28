package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// goneSummary marks a diagnostic that means "the object is not there".
//
// The framework has no sentinel for it, and a resource needs to tell that case
// apart from a real failure so Read can drop the resource from state rather
// than fail the run.
const goneSummary = "__gone__"

// isGone reports whether the diagnostics carry the not-found marker.
func isGone(diags diag.Diagnostics) bool {
	for _, d := range diags {
		if d.Summary() == goneSummary {
			return true
		}
	}
	return false
}

// cryptoKeyID builds the composite id.
func cryptoKeyID(zone string, keyID int) string {
	return zone + "/" + strconv.Itoa(keyID)
}

// readInto refreshes the model from the **collection** endpoint.
//
// Never GetCryptoKey. That call returns the private key, and a resource that
// used it would put key material one careless assignment away from state. The
// collection carries everything this resource exposes — dnskey, ds, active,
// published, algorithm, bits — and omits the one field it must not have.
//
// mode follows the same rule as applyZone: after a write, attributes the
// configuration set keep their planned values, because Terraform requires a
// create to return exactly what was planned.
func (r *cryptoKeyResource) readInto(
	ctx context.Context,
	zoneID string,
	keyID int,
	model *cryptoKeyModel,
	mode applyMode,
) diag.Diagnostics {
	var diags diag.Diagnostics

	client, d := r.clients.RequireAuth("powerdns_zone_cryptokey")
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	keys, err := client.ListCryptoKeys(ctx, zoneID)
	if err != nil {
		// A deleted zone takes its keys with it, and that is a removal rather
		// than an error.
		diags.AddError(goneSummary, err.Error())
		return diags
	}

	var found *cryptoKeyFields
	for i := range keys {
		if keys[i].ID != keyID {
			continue
		}
		if keys[i].PrivateKey != "" {
			// The collection is documented to omit it. If that ever changes,
			// this resource is no longer safe and must not carry on quietly.
			diags.AddError(
				"PowerDNS returned key material from the collection endpoint",
				"GET /zones/"+zoneID+"/cryptokeys included a private key, which it "+
					"has never done before. The provider refuses to continue rather "+
					"than risk writing it to state. Please report this.",
			)
			return diags
		}
		found = &cryptoKeyFields{
			KeyType:   keys[i].KeyType,
			Algorithm: keys[i].Algorithm,
			Bits:      keys[i].Bits,
			Active:    keys[i].Active,
			Published: derefBool(keys[i].Published),
			DNSKey:    keys[i].DNSKey,
			DS:        keys[i].DS,
		}
		break
	}

	if found == nil {
		diags.AddError(goneSummary, "the key is no longer present in the zone")
		return diags
	}

	model.DNSKey = types.StringValue(found.DNSKey)
	model.Bits = types.Int64Value(int64(found.Bits))

	ds, listDiags := types.ListValueFrom(ctx, types.StringType, found.DS)
	diags.Append(listDiags...)
	if diags.HasError() {
		return diags
	}
	model.DS = ds

	if mode == afterRead {
		model.KeyType = types.StringValue(found.KeyType)
		model.Active = types.BoolValue(found.Active)
		model.Published = types.BoolValue(found.Published)
	}

	// Algorithm is Optional and Computed: the server picks one when the
	// configuration is silent, and a value the operator set must be preserved.
	if mode == afterRead || model.Algorithm.IsUnknown() || model.Algorithm.IsNull() {
		model.Algorithm = types.StringValue(found.Algorithm)
	}

	return diags
}

// cryptoKeyFields is what the collection endpoint gives, and deliberately has
// no field for key material.
type cryptoKeyFields struct {
	KeyType   string
	Algorithm string
	Bits      int
	Active    bool
	Published bool
	DNSKey    string
	DS        []string
}
