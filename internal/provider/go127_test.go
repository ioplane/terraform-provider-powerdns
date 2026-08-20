package provider_test

import "testing"

type genericMethodFixture struct{}

func (genericMethodFixture) identity[T any](value T) T { return value }

type embeddedFixture struct {
	Value string
}

type selectorKeyFixture struct {
	embeddedFixture
}

func inferredIdentity[T any](value T) T { return value }

func invokeInferred(identity func(string) string, value string) string {
	return identity(value)
}

func TestGo127LanguageContract(t *testing.T) {
	t.Parallel()
	if got := (genericMethodFixture{}).identity("powerdns"); got != "powerdns" {
		t.Fatalf("generic method = %q", got)
	}
	keyed := selectorKeyFixture{Value: "selector"}
	if keyed.Value != "selector" {
		t.Fatalf("promoted selector key = %q", keyed.Value)
	}
	if got := invokeInferred(inferredIdentity, "inferred"); got != "inferred" {
		t.Fatalf("inferred function = %q", got)
	}
}
