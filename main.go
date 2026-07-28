// Command terraform-provider-powerdns serves the PowerDNS provider.
//
// The provider covers the PowerDNS family — Authoritative Server, Recursor and
// dnsdist — over their HTTP APIs. See docs/adr/0002-one-provider-for-the-family.md
// for why they are one provider, and docs/adr/0003-framework-protocol-6.md for
// why this is terraform-plugin-framework rather than SDKv2.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/ioplane/terraform-provider-powerdns/internal/provider"
)

// registryAddress must match the source in a consumer's required_providers
// block.
const registryAddress = "registry.terraform.io/ioplane/powerdns"

// version is set by the linker at release time; see the goreleaser ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false,
		"run with support for a debugger such as delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: registryAddress,
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatalf("serving the provider: %v", err)
	}
}
