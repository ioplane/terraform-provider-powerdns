package normalise_test

import (
	"strconv"
	"testing"

	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
)

func BenchmarkStringMultiset(b *testing.B) {
	for _, benchmark := range []struct {
		name  string
		value func(int) string
		key   func(string) string
	}{
		{"dns", func(index int) string { return "host-" + strconv.Itoa(index) + ".example." }, normalise.DNSNameKey},
		{"acl", func(index int) string {
			return "10." + strconv.Itoa(index/256) + "." + strconv.Itoa(index%256) + ".1/24"
		}, normalise.CIDRKey},
		{"rrset-a", func(index int) string {
			return "192.0." + strconv.Itoa(index/256) + "." + strconv.Itoa(index%256)
		}, func(value string) string { return normalise.RecordContentKey("A", value) }},
	} {
		for _, size := range []int{8, 1_024} {
			benchmarkStringMultiset(b, benchmark.name+"/"+strconv.Itoa(size), size, benchmark.value, benchmark.key)
		}
	}
}

func benchmarkStringMultiset(
	b *testing.B,
	name string,
	size int,
	value func(int) string,
	key func(string) string,
) {
	b.Helper()
	values := make([]string, size)
	reversed := make([]string, size)
	for index := range size {
		values[index] = value(index)
		reversed[size-index-1] = values[index]
	}
	b.Run(name, func(b *testing.B) {
		for b.Loop() {
			if !normalise.StringMultiset(values, reversed, key) {
				b.Fatal("permutation did not compare equal")
			}
		}
	})
}
