package testutil_test

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/testutil"
)

// TestSpecIsLoadable is the only thing about the vendored specification that is
// this repository's problem. If it stops loading, the file is broken here.
func TestSpecIsLoadable(t *testing.T) {
	t.Parallel()

	checker, err := testutil.NewSpecChecker(context.Background(), ".")
	if err != nil {
		t.Fatalf("the vendored specification does not load: %v", err)
	}

	ops := checker.DocumentedOperations()
	if len(ops) == 0 {
		t.Fatal("the specification documents no operations")
	}
	t.Logf("the specification documents %d operations", len(ops))
}

// TestRecordedResponsesMatchTheSpec cross-checks every Authoritative fixture.
//
// It fails on a schema mismatch, which is drift worth investigating, and only
// reports a route the specification does not describe — the specification is
// demonstrably incomplete, and failing on that would make the check
// permanently red and therefore ignored.
func TestRecordedResponsesMatchTheSpec(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	checker, err := testutil.NewSpecChecker(ctx, ".")
	if err != nil {
		t.Fatalf("NewSpecChecker: %v", err)
	}

	fixtures, err := testutil.Load(".", testutil.ProductAuth)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fixtures) == 0 {
		t.Skip("no Authoritative fixtures recorded yet; run task fixtures:record")
	}

	var matched, unmatched int
	for _, f := range fixtures {
		result := checker.Check(ctx, f)

		switch {
		case result.Err != nil:
			t.Errorf("%s: the recorded response does not match the specification: %v",
				f.Name, result.Err)
		case result.Matched:
			matched++
		case result.Divergence != nil:
			t.Logf("%s: known divergence — %s", f.Name, result.Divergence.Reason)
			unmatched++
		default:
			t.Logf("%s: %s %s is not described by the specification",
				f.Name, f.Method, f.Path)
			unmatched++
		}
	}

	t.Logf("%d fixture(s) matched the specification, %d not described by it",
		matched, unmatched)
}

// TestNoSpecForRecursorOrDNSDist records the reason those products have no
// cross-check, so the absence reads as a property of PowerDNS rather than as
// something missing here.
func TestNoSpecForRecursorOrDNSDist(t *testing.T) {
	t.Parallel()

	for _, product := range []testutil.Product{testutil.ProductRec, testutil.ProductDNSDist} {
		fixtures, err := testutil.Load(".", product)
		if err != nil {
			t.Fatalf("Load(%s): %v", product, err)
		}
		for _, f := range fixtures {
			if f.Status == 0 || f.Method == "" || f.Path == "" {
				t.Errorf("%s/%s: incomplete fixture", product, f.Name)
			}
			if f.RecordedAgainst == "" {
				t.Errorf("%s/%s: no recorded_against version, so staleness is invisible",
					product, f.Name)
			}
		}
		t.Logf("%s: %d fixture(s), well-formedness only — PowerDNS publishes no specification",
			product, len(fixtures))
	}
}
