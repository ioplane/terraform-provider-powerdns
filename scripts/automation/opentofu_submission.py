"""Print the two OpenTofu Registry submission links, prefilled.

The OpenTofu Registry takes providers through GitHub issue forms and refuses
anything else — the templates say so twice, and submissions made through the
API or `gh` are closed unprocessed. So this cannot submit; it can only make the
manual step short and unambiguous by prefilling every field the form accepts as
a query parameter.

The signing key is read back from the Terraform Registry rather than from a
file here. That copy is the one already serving the published provider, so the
key submitted to OpenTofu is by construction the key Terraform users verify
against — and no key material lives in this repository.

Two fields cannot be prefilled and are left to the person submitting: the
public-membership checkbox on the key form, which is a declaration and not
data, and the issue body's confirmation step.

Run as: python -m scripts.automation.opentofu_submission
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

NAMESPACE = "ioplane"
PROVIDER = "powerdns"
REPOSITORY = f"{NAMESPACE}/terraform-provider-{PROVIDER}"
REGISTRY = f"https://registry.terraform.io/v1/providers/{NAMESPACE}/{PROVIDER}"
FORMS = "https://github.com/opentofu/registry/issues/new"
TIMEOUT = 20.0

# The registry that consumes this key accepts RSA and DSA only. An ECC key is
# accepted by the form and rejected later, at a point far away from here.
ACCEPTED_ALGORITHMS = {"1": "RSA", "17": "DSA"}


def get_json(url: str) -> dict | None:
    """GET a JSON document, returning None if it did not answer."""
    try:
        with urllib.request.urlopen(url, timeout=TIMEOUT) as response:  # noqa: S310  # nosemgrep
            return json.load(response)
    except (OSError, ValueError):
        return None


def key_algorithm(armor: str) -> str | None:
    """Return the GnuPG algorithm number of an armoured public key."""
    if shutil.which("gpg") is None:
        return None
    result = subprocess.run(
        ["gpg", "--show-keys", "--with-colons"],
        input=armor,
        capture_output=True,
        text=True,
        check=False,
    )
    for line in result.stdout.splitlines():
        fields = line.split(":")
        if fields and fields[0] == "pub":
            return fields[3]
    return None


def form_url(**fields: str) -> str:
    """Build a prefilled issue-form URL."""
    return f"{FORMS}?{urllib.parse.urlencode(fields)}"


def main() -> int:
    """Print the two links, after confirming what they would submit."""
    print("== the published version, and the key that signed it ==")
    published = get_json(REGISTRY)
    if published is None or "version" not in published:
        print(
            "FAIL  the provider is not published on the Terraform Registry yet",
            file=sys.stderr,
        )
        return 1
    version = published["version"]
    print(f"ok    {REPOSITORY} {version} is published")

    download = get_json(f"{REGISTRY}/{version}/download/linux/amd64")
    keys = ((download or {}).get("signing_keys") or {}).get("gpg_public_keys") or []
    if not keys:
        print(
            f"FAIL  the registry served no signing key for {version}", file=sys.stderr
        )
        return 1
    armor = keys[0]["ascii_armor"]

    algorithm = key_algorithm(armor)
    if algorithm is None:
        print("WARN  could not read the key algorithm", file=sys.stderr)
    elif algorithm in ACCEPTED_ALGORITHMS:
        print(f"ok    signing key is {ACCEPTED_ALGORITHMS[algorithm]}")
    else:
        print(
            f"FAIL  key algorithm {algorithm} is neither RSA nor DSA; "
            "the registry accepts only those",
            file=sys.stderr,
        )
        return 1

    key_link = form_url(
        labels="provider-key,submission",
        template="provider_key.yml",
        title=f"Provider Key: {NAMESPACE}/{PROVIDER}",
        namespace=NAMESPACE,
        providername=PROVIDER,
        gpgkey=armor,
    )
    provider_link = form_url(
        labels="provider,submission",
        template="provider.yml",
        title=f"Provider: {REPOSITORY}",
        repository=REPOSITORY,
    )

    print(f"""
Open these in a browser, in this order. The key must be known before the
provider's versions can have their signatures checked.

1. Signing key — tick "I have made my membership public", then submit:

{key_link}

2. Provider:

{provider_link}

Neither can be submitted from a terminal: the registry's issue templates state
that submissions made outside the issue form UI are closed unprocessed.""")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
