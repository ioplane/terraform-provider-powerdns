package testutil_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

// secretFields are the JSON keys PowerDNS uses for key material.
//
// privatekey is the DNSSEC private key, returned by GET on a single cryptokey
// and by the create response. key is the TSIG shared secret, returned by GET
// on a single tsigkey and by its create response. Both are blank or absent in
// the corresponding collection responses, which is why recording a list is
// safe and recording a get is not.
var secretFields = []string{"privatekey", "key", "secret", "password"}

// TestFixturesCarryNoKeyMaterial fails if a recorded fixture holds a secret.
//
// The golden rule in AGENTS.md is that no secret reaches state. Fixtures are
// not state, but they are committed, and a fixture recorded from
// GET /cryptokeys/{id} would put a DNSSEC private key in git permanently —
// where removing it later means rewriting history, not deleting a file.
//
// The recorder is careful today. This is what keeps it careful when somebody
// records "just one more fixture" against a single-key endpoint.
func TestFixturesCarryNoKeyMaterial(t *testing.T) {
	t.Parallel()

	products := []testutil.Product{
		testutil.ProductAuth, testutil.ProductRec, testutil.ProductDNSDist,
	}

	var checked int
	for _, product := range products {
		fixtures, err := testutil.Load(".", product)
		if err != nil {
			t.Fatalf("Load(%s): %v", product, err)
		}

		for _, f := range fixtures {
			checked++
			if len(f.Body) == 0 {
				continue
			}

			var body any
			if err := json.Unmarshal(f.Body, &body); err != nil {
				t.Errorf("%s/%s: body is not valid JSON: %v", product, f.Name, err)
				continue
			}

			for _, found := range findSecrets(body, "") {
				t.Errorf("%s/%s holds key material at %s.\n"+
					"Re-record against the collection endpoint, which blanks it, "+
					"and if this is already committed the fix is to rewrite history, "+
					"not to delete the file.",
					product, f.Name, found)
			}
		}
	}

	if checked == 0 {
		t.Fatal("no fixtures were checked; the loader found nothing")
	}
	t.Logf("%d fixture(s) checked for key material", checked)
}

// findSecrets walks decoded JSON and returns the paths of non-empty fields
// whose name marks them as key material.
func findSecrets(node any, path string) []string {
	var found []string

	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			here := filepath.Join(path, key)
			if isSecretField(key) && !isEmpty(child) {
				found = append(found, here)
			}
			found = append(found, findSecrets(child, here)...)
		}
	case []any:
		for i, child := range v {
			found = append(found, findSecrets(child, filepath.Join(path, strconv.Itoa(i)))...)
		}
	}
	return found
}

func isSecretField(name string) bool {
	return slices.Contains(secretFields, strings.ToLower(name))
}

// isEmpty reports whether a value carries nothing worth protecting. A blank
// string is what a collection response puts in the TSIG key field, and
// flagging it would make the check fire on every safe fixture.
func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	}
	return false
}
