# Security policy

## Reporting

Report privately through GitHub Security Advisories on this repository. Do not
open a public issue for an unfixed vulnerability.

A flaw in PowerDNS itself goes to the PowerDNS security team, not here. This is
a client.

## What this provider handles

API keys for three web servers, TSIG secrets, and DNSSEC private key material.

### The state-file problem

Terraform state is **not encrypted**. Any attribute the provider reads back is
written to state in plain text, including attributes marked `Sensitive` —
`Sensitive` redacts console output and nothing more.

The response, in order:

1. **Write-only attributes** (Terraform 1.11+) for values needing no drift
   detection.
2. **Ephemeral resources** for values that exist only for the duration of a
   run — this is where DNSSEC private keys and generated TSIG secrets live.
3. Where neither applies, the resource documentation states plainly that the
   value lands in state.

A DNSSEC private key is never an attribute of a managed resource. This is
tested, not asserted: an acceptance test reads the state file and fails if it
finds key material.

## Supply chain

- Container images pinned by `sha256` digest; GitHub Actions by commit SHA.
  `scripts/checks/pins.py` rejects a floating or unresolvable reference.
- Go dependencies pinned exactly, verified by `go.sum`.
- `govulncheck`, `osv-scanner` and `semgrep` run in CI.
- Release archives are GPG-signed.

## Lab credentials

`labapikey` is a deliberately public test value in a fixture bound to loopback.
It is not a secret and is never reused. Never point acceptance tests at a
production PowerDNS.
