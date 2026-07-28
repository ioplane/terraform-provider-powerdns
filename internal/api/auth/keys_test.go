package auth_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
)

// TestListCryptoKeys_CarriesNoPrivateKey is the security-relevant one.
//
// PowerDNS omits privatekey from the collection and includes it in the
// single-key response. Anything reconciling state must read the collection, so
// this pins the property the resource layer will depend on. If PowerDNS ever
// starts sending it here, this fails and the design has to change before a
// private key reaches anybody's state file.
func TestListCryptoKeys_CarriesNoPrivateKey(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	keys, err := client.ListCryptoKeys(context.Background(), "s202.test.")
	if err != nil {
		t.Fatalf("ListCryptoKeys: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("no keys in the fixture")
	}

	for _, k := range keys {
		if k.PrivateKey != "" {
			t.Errorf("key %d carries private material in a list response; "+
				"the reconcile path is no longer safe", k.ID)
		}
		if k.DNSKey == "" {
			t.Errorf("key %d has no dnskey; the list is not usable for reconciliation", k.ID)
		}
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost/zones/s202.test./cryptokeys")
}

// TestListCryptoKeys_KSKHasDSAndZSKDoesNot records a shape that reads like a
// failed request and is not: a ZSK has no delegation signer.
func TestListCryptoKeys_KSKHasDSAndZSKDoesNot(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	keys, err := client.ListCryptoKeys(context.Background(), "s202.test.")
	if err != nil {
		t.Fatalf("ListCryptoKeys: %v", err)
	}

	for _, k := range keys {
		switch k.KeyType {
		case auth.KeyTypeKSK:
			if len(k.DS) == 0 {
				t.Errorf("KSK %d has no DS records", k.ID)
			}
		case auth.KeyTypeZSK:
			if len(k.DS) != 0 {
				t.Errorf("ZSK %d has DS records, which it should not", k.ID)
			}
		}
	}
}

// TestListTSIGKeys_BlanksTheSecret is the TSIG half of the same property.
// Note the difference from cryptokeys: the field is present and empty rather
// than omitted, so a caller checking for presence rather than emptiness would
// conclude it had the secret.
func TestListTSIGKeys_BlanksTheSecret(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	keys, err := client.ListTSIGKeys(context.Background())
	if err != nil {
		t.Fatalf("ListTSIGKeys: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("no TSIG keys in the fixture")
	}

	for _, k := range keys {
		if k.Key != "" {
			t.Errorf("tsigkey %q carries its secret in a list response", k.ID)
		}
	}
}

// TestTSIGKeyIDIsCanonicalised records a normalisation that would otherwise
// surface as a 404 on the read after a successful create: a key requested as
// "probe" gets the id "probe.".
func TestTSIGKeyIDIsCanonicalised(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	keys, err := client.ListTSIGKeys(context.Background())
	if err != nil {
		t.Fatalf("ListTSIGKeys: %v", err)
	}

	for _, k := range keys {
		if !strings.HasSuffix(k.ID, ".") {
			t.Errorf("tsigkey id %q has no trailing dot; the canonical form is expected "+
				"and a caller keying on the requested name would 404", k.ID)
		}
	}
}

// TestListMetadata_IncludesWhatTheServerAssigned warns the resource layer.
// A zone nobody has touched already has one metadata entry, so treating the
// whole list as managed state means trying to delete SOA-EDIT-API.
func TestListMetadata_IncludesWhatTheServerAssigned(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	entries, err := client.ListMetadata(context.Background(), "s202.test.")
	if err != nil {
		t.Fatalf("ListMetadata: %v", err)
	}

	var found bool
	for _, e := range entries {
		if e.Kind == "SOA-EDIT-API" {
			found = true
		}
	}
	if !found {
		t.Error("SOA-EDIT-API is absent; the server assigns it on zone creation and " +
			"the resource layer is written expecting it")
	}
}

// TestGetMetadata_UnsetKindIsEmptyNot404 pins the distinction the resource
// layer needs: absence and emptiness are the same state here, so a read of an
// unset kind must not be mistaken for a missing zone.
func TestGetMetadata_UnsetKindIsEmptyNot404(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	m, err := client.GetMetadata(context.Background(), "s202.test.", "ALLOW-DNSUPDATE-FROM")
	if err != nil {
		t.Fatalf("an unset kind must not be an error: %v", err)
	}
	if len(m.Metadata) != 0 {
		t.Errorf("expected no values, got %v", m.Metadata)
	}
	if m.Kind != "ALLOW-DNSUPDATE-FROM" {
		t.Errorf("Kind = %q, want the kind that was asked for", m.Kind)
	}
}

func TestListAutoprimaries(t *testing.T) {
	t.Parallel()

	client, mock := newFixtureClient(t)

	aps, err := client.ListAutoprimaries(context.Background())
	if err != nil {
		t.Fatalf("ListAutoprimaries: %v", err)
	}
	if len(aps) == 0 {
		t.Fatal("no autoprimaries in the fixture")
	}
	if aps[0].IP == "" || aps[0].Nameserver == "" {
		t.Error("ip and nameserver are the composite key and must both be present")
	}
	mock.AssertCalled(http.MethodGet, "/api/v1/servers/localhost/autoprimaries")
}

// TestDeleteAutoprimary_BuildsTheCompositePath checks the two-segment delete
// path. There is no id: the pair is the key.
func TestDeleteAutoprimary_BuildsTheCompositePath(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	err := client.DeleteAutoprimary(context.Background(), "192.0.2.53", "ns1.probe.test.")
	if err != nil {
		t.Fatalf("DeleteAutoprimary: %v", err)
	}

	want := "/api/v1/servers/localhost/autoprimaries/192.0.2.53/ns1.probe.test."
	if got := (*seen)[0].RequestURI; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

// TestCryptoKeyPathUsesTheIntegerID guards against the id being formatted as
// anything other than a plain integer.
func TestCryptoKeyPathUsesTheIntegerID(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	if err := client.DeleteCryptoKey(context.Background(), "s202.test.", 7); err != nil {
		t.Fatalf("DeleteCryptoKey: %v", err)
	}

	want := "/api/v1/servers/localhost/zones/s202.test./cryptokeys/7"
	if got := (*seen)[0].RequestURI; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

// TestSetCryptoKeyState_SendsOnlyTheMutableFields checks that a state change
// carries active and published and nothing else. PowerDNS ignores the rest,
// so sending a whole key would look like it worked and change only these two.
func TestSetCryptoKeyState_SendsOnlyTheMutableFields(t *testing.T) {
	t.Parallel()

	client, seen := newRecordingClient(t)

	published := true
	if err := client.SetCryptoKeyState(context.Background(), "s202.test.", 1, false, &published); err != nil {
		t.Fatalf("SetCryptoKeyState: %v", err)
	}

	body := (*seen)[0].Body
	for _, forbidden := range []string{"privatekey", "dnskey", "algorithm", "keytype"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the state change carried %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"active":false`) {
		t.Errorf("active did not reach the request: %s", body)
	}
}

// TestPostCryptoKey_IsTheUndocumentedOperation exercises defect 2 of
// PowerDNS/pdns#17807 — the route exists and answers 400, where an absent
// route would answer 404. That difference is how the gap was found.
func TestPostCryptoKey_IsTheUndocumentedOperation(t *testing.T) {
	t.Parallel()

	client, _ := newFixtureClient(t)

	err := client.RectifyAfterKeyChange(context.Background(), "s202.test.", 1)
	if err == nil {
		t.Fatal("expected an error; the operation rejects an empty body")
	}
	if strings.Contains(err.Error(), "404") {
		t.Errorf("a 404 would mean the route does not exist, which would contradict "+
			"the finding reported upstream: %v", err)
	}
}
