package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

// Provider functions do the name arithmetic that otherwise gets smeared across
// resources.
//
// Every one of these is pure and offline: no client, no request, no state.
// That is why they are functions rather than data sources — a data source
// would make a plan depend on a server for an answer that is a string
// operation.
//
// The alternative is what other DNS providers do: `join(".", reverse(split(".",
// var.subnet)))` in a locals block, copied between modules and subtly wrong for
// IPv6 or for a prefix that is not on a byte boundary.

var (
	_ function.Function = (*fqdnFunction)(nil)
	_ function.Function = (*isFQDNFunction)(nil)
	_ function.Function = (*reverseZoneNameFunction)(nil)
	_ function.Function = (*ptrNameFunction)(nil)
	_ function.Function = (*soaSerialFunction)(nil)
)

// fqdnFunction appends the trailing dot PowerDNS stores.
type fqdnFunction struct{}

// NewFQDNFunction returns the function factory.
func NewFQDNFunction() function.Function { return &fqdnFunction{} }

func (f *fqdnFunction) Metadata(
	_ context.Context,
	_ function.MetadataRequest,
	resp *function.MetadataResponse,
) {
	resp.Name = "fqdn"
}

func (f *fqdnFunction) Definition(
	_ context.Context,
	_ function.DefinitionRequest,
	resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "Makes a name fully qualified.",
		MarkdownDescription: "Appends a trailing dot if absent, which is the form " +
			"PowerDNS stores and the form its ids take.\n\n" +
			"```terraform\n" +
			"provider::powerdns::fqdn(\"example.com\")  # \"example.com.\"\n" +
			"provider::powerdns::fqdn(\"example.com.\") # \"example.com.\"\n" +
			"```",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "name",
				MarkdownDescription: "The name to qualify.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *fqdnFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var name string
	resp.Error = function.ConcatFuncErrors(resp.Error,
		req.Arguments.Get(ctx, &name))
	if resp.Error != nil {
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error,
		resp.Result.Set(ctx, canonicalName(name)))
}

// isFQDNFunction reports whether a name already carries the trailing dot.
type isFQDNFunction struct{}

// NewIsFQDNFunction returns the function factory.
func NewIsFQDNFunction() function.Function { return &isFQDNFunction{} }

func (f *isFQDNFunction) Metadata(
	_ context.Context,
	_ function.MetadataRequest,
	resp *function.MetadataResponse,
) {
	resp.Name = "is_fqdn"
}

func (f *isFQDNFunction) Definition(
	_ context.Context,
	_ function.DefinitionRequest,
	resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "Reports whether a name is fully qualified.",
		MarkdownDescription: "True when the name ends in a dot. The empty string is " +
			"false: it is not a name at all, let alone a qualified one.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "name",
				MarkdownDescription: "The name to test.",
			},
		},
		Return: function.BoolReturn{},
	}
}

func (f *isFQDNFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var name string
	resp.Error = function.ConcatFuncErrors(resp.Error,
		req.Arguments.Get(ctx, &name))
	if resp.Error != nil {
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error,
		resp.Result.Set(ctx, name != "" && strings.HasSuffix(name, ".")))
}

// reverseZoneNameFunction turns a subnet into the zone that holds its PTRs.
type reverseZoneNameFunction struct{}

// NewReverseZoneNameFunction returns the function factory.
func NewReverseZoneNameFunction() function.Function { return &reverseZoneNameFunction{} }

func (f *reverseZoneNameFunction) Metadata(
	_ context.Context,
	_ function.MetadataRequest,
	resp *function.MetadataResponse,
) {
	resp.Name = "reverse_zone_name"
}

func (f *reverseZoneNameFunction) Definition(
	_ context.Context,
	_ function.DefinitionRequest,
	resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "The reverse zone for a subnet.",
		MarkdownDescription: "Builds the `in-addr.arpa` or `ip6.arpa` zone name for a " +
			"CIDR prefix.\n\n" +
			"```terraform\n" +
			"provider::powerdns::reverse_zone_name(\"192.0.2.0/24\")\n" +
			"# \"2.0.192.in-addr.arpa.\"\n" +
			"provider::powerdns::reverse_zone_name(\"2001:db8::/32\")\n" +
			"# \"8.b.d.0.1.0.0.2.ip6.arpa.\"\n" +
			"```\n\n" +
			"IPv4 prefixes must be on an octet boundary — /8, /16 or /24 — and IPv6 " +
			"prefixes on a nibble boundary, a multiple of 4. Anything else has no " +
			"single reverse zone, and this reports that rather than guessing.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "cidr",
				MarkdownDescription: "The subnet, in CIDR form.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *reverseZoneNameFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var cidr string
	resp.Error = function.ConcatFuncErrors(resp.Error,
		req.Arguments.Get(ctx, &cidr))
	if resp.Error != nil {
		return
	}

	zone, err := reverseZoneName(cidr)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error,
			function.NewArgumentFuncError(0, err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, zone))
}

// ptrNameFunction turns an address into the name its PTR record sits at.
type ptrNameFunction struct{}

// NewPTRNameFunction returns the function factory.
func NewPTRNameFunction() function.Function { return &ptrNameFunction{} }

func (f *ptrNameFunction) Metadata(
	_ context.Context,
	_ function.MetadataRequest,
	resp *function.MetadataResponse,
) {
	resp.Name = "ptr_name"
}

func (f *ptrNameFunction) Definition(
	_ context.Context,
	_ function.DefinitionRequest,
	resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "The PTR name for an address.",
		MarkdownDescription: "```terraform\n" +
			"provider::powerdns::ptr_name(\"192.0.2.1\")\n" +
			"# \"1.2.0.192.in-addr.arpa.\"\n" +
			"provider::powerdns::ptr_name(\"2001:db8::1\")\n" +
			"# \"1.0.0.….8.b.d.0.1.0.0.2.ip6.arpa.\"\n" +
			"```",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "address",
				MarkdownDescription: "The IPv4 or IPv6 address.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *ptrNameFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var address string
	resp.Error = function.ConcatFuncErrors(resp.Error,
		req.Arguments.Get(ctx, &address))
	if resp.Error != nil {
		return
	}

	name, err := ptrName(address)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error,
			function.NewArgumentFuncError(0, err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, name))
}

// soaSerialFunction builds a serial in the date-and-counter convention.
type soaSerialFunction struct{}

// NewSOASerialFunction returns the function factory.
func NewSOASerialFunction() function.Function { return &soaSerialFunction{} }

func (f *soaSerialFunction) Metadata(
	_ context.Context,
	_ function.MetadataRequest,
	resp *function.MetadataResponse,
) {
	resp.Name = "soa_serial"
}

func (f *soaSerialFunction) Definition(
	_ context.Context,
	_ function.DefinitionRequest,
	resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "A SOA serial in YYYYMMDDnn form.",
		MarkdownDescription: "Builds the conventional date-and-counter serial from a " +
			"date and a revision.\n\n" +
			"```terraform\n" +
			"provider::powerdns::soa_serial(\"2026-07-29\", 1) # 2026072901\n" +
			"```\n\n" +
			"The date is taken as an argument rather than read from the clock, because " +
			"a function that returned today's date would change the plan every day and " +
			"never converge.\n\n" +
			"The revision is 0 to 99. A hundredth change on one day has outgrown this " +
			"convention, and silently rolling into the next day's serial would be " +
			"worse than saying so.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "date",
				MarkdownDescription: "The date, as `YYYY-MM-DD`.",
			},
			function.Int64Parameter{
				Name:                "revision",
				MarkdownDescription: "The revision within that day, 0 to 99.",
			},
		},
		Return: function.Int64Return{},
	}
}

func (f *soaSerialFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var (
		date     string
		revision int64
	)
	resp.Error = function.ConcatFuncErrors(resp.Error,
		req.Arguments.Get(ctx, &date, &revision))
	if resp.Error != nil {
		return
	}

	serial, err := soaSerial(date, revision)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error,
			function.NewFuncError(err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, serial))
}

// The three refusals these functions make, as sentinels.
//
// Each is a rule of the DNS rather than a transient failure, so a caller that
// wants to branch on which rule was broken can, and the message stays free to
// say why in full.
var (
	// ErrPrefixOffBoundary is a prefix that spans several reverse zones.
	ErrPrefixOffBoundary = errors.New("the prefix is not on a reverse-zone boundary")
	// ErrRevisionOutOfRange is a SOA revision the convention cannot hold.
	ErrRevisionOutOfRange = errors.New("the revision does not fit the YYYYMMDDnn convention")
)

// reverseZoneName is the logic behind the function, kept separate so it can be
// tested without a Terraform runtime.
func reverseZoneName(cidr string) (string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", fmt.Errorf("%q is not a CIDR prefix: %w", cidr, err)
	}
	prefix = prefix.Masked()

	if prefix.Addr().Is4() {
		if prefix.Bits()%8 != 0 {
			return "", fmt.Errorf(
				"%w: an IPv4 reverse zone needs an octet boundary — /8, /16 or /24 — "+
					"and %q is /%d. A prefix between boundaries spans several reverse "+
					"zones, so there is no single name to return",
				ErrPrefixOffBoundary, cidr, prefix.Bits())
		}
		octets := strings.Split(prefix.Addr().String(), ".")
		kept := prefix.Bits() / 8

		labels := make([]string, 0, kept)
		for i := kept - 1; i >= 0; i-- {
			labels = append(labels, octets[i])
		}
		return joinReverseLabels(labels, "in-addr.arpa."), nil
	}

	if prefix.Bits()%4 != 0 {
		return "", fmt.Errorf(
			"%w: an IPv6 reverse zone needs a nibble boundary — a multiple of 4 — "+
				"and %q is /%d", ErrPrefixOffBoundary, cidr, prefix.Bits())
	}

	nibbles := hexNibbles(prefix.Addr())
	kept := prefix.Bits() / 4

	labels := make([]string, 0, kept)
	for i := kept - 1; i >= 0; i-- {
		labels = append(labels, nibbles[i])
	}
	return joinReverseLabels(labels, "ip6.arpa."), nil
}

// joinReverseLabels attaches the labels to the suffix.
//
// /0 keeps no labels — it is the root of the reverse tree — and naive
// concatenation would produce ".in-addr.arpa." with a leading dot, which is a
// different name and not a valid one.
func joinReverseLabels(labels []string, suffix string) string {
	if len(labels) == 0 {
		return suffix
	}
	return strings.Join(labels, ".") + "." + suffix
}

// ptrName is the logic behind the function.
func ptrName(address string) (string, error) {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return "", fmt.Errorf("%q is not an IP address: %w", address, err)
	}

	if addr.Is4() {
		octets := strings.Split(addr.String(), ".")
		return fmt.Sprintf("%s.%s.%s.%s.in-addr.arpa.",
			octets[3], octets[2], octets[1], octets[0]), nil
	}

	nibbles := hexNibbles(addr)
	labels := make([]string, 0, len(nibbles))
	for _, nibble := range slices.Backward(nibbles) {
		labels = append(labels, nibble)
	}
	return joinReverseLabels(labels, "ip6.arpa."), nil
}

// hexNibbles expands an address into its 32 hexadecimal nibbles, most
// significant first.
//
// An IPv4-mapped IPv6 address is expanded as IPv6, because that is what it is:
// ::ffff:192.0.2.1 lives under ip6.arpa, not in-addr.arpa.
func hexNibbles(addr netip.Addr) []string {
	// As16 already maps an IPv4 address into ::ffff:a.b.c.d, which is what an
	// IPv4-mapped address should expand to. A bare IPv4 address never reaches
	// here — the callers handle it before.
	bytes := addr.As16()

	nibbles := make([]string, 0, 32)
	for _, b := range bytes {
		nibbles = append(nibbles,
			strconv.FormatUint(uint64(b>>4), 16),
			strconv.FormatUint(uint64(b&0x0f), 16),
		)
	}
	return nibbles
}

// soaSerial is the logic behind the function.
func soaSerial(date string, revision int64) (int64, error) {
	const layout = "2006-01-02"

	parsed, err := time.Parse(layout, date)
	if err != nil {
		return 0, fmt.Errorf("%q is not a date in YYYY-MM-DD form: %w", date, err)
	}

	if revision < 0 || revision > 99 {
		return 0, fmt.Errorf(
			"%w: it must be 0 to 99 and is %d. A hundredth change in one day has "+
				"outgrown the convention, and rolling into the next day's serial "+
				"silently would be worse than this error",
			ErrRevisionOutOfRange, revision)
	}

	base, err := strconv.ParseInt(parsed.Format("20060102"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("formatting %q: %w", date, err)
	}

	return base*100 + revision, nil
}
