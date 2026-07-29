#!/usr/bin/env bash
# Asserts that a built release is shaped the way the Terraform Registry
# ingests, before it is a release.
#
# This exists because v0.1.0 was built, signed, published and then refused.
# The Registry parses SHA256SUMS and treats every line as a file belonging to
# the version; adding SBOMs to the release put thirteen lines in there that it
# will not accept, and it rejected the whole submission with "missing files in
# request body". Nothing in the release path noticed, because every check asked
# whether the artefacts were correct and none asked whether they were the ones
# the Registry expects.
#
# Usage: check-release-artifacts.sh [dist-dir]
set -euo pipefail

readonly DIST="${1:-dist}"
declare -i bad=0

fail() {
  printf 'FAIL  %s\n' "$1" >&2
  bad+=1
}
pass() { printf 'ok    %s\n' "$1"; }

sums="$(find "$DIST" -maxdepth 1 -name '*_SHA256SUMS' -print -quit)"
if [[ -z "$sums" ]]; then
  echo "no *_SHA256SUMS in ${DIST}; build first (task release:dryrun)" >&2
  exit 2
fi

echo "== SHA256SUMS lists only what the Registry ingests =="
# Exactly two shapes: the platform archives, and the manifest. Anything else —
# an SBOM, a signature, a checksum of a checksum — is a line the Registry
# cannot resolve to a file it accepts.
while read -r name; do
  [[ -n "$name" ]] || continue
  case "$name" in
    *_manifest.json) pass "manifest    ${name}" ;;
    *.zip) pass "archive     ${name}" ;;
    *) fail "${name} is neither an archive nor the manifest" ;;
  esac
done < <(awk '{print $2}' "$sums")

echo
echo "== every archive matches its recorded digest =="
# Only the archives live in dist. The manifest is checksummed under a renamed
# copy that exists in the uploaded release and never on disk, so it is checked
# against its source below rather than here.
if (cd "$DIST" && grep '\.zip$' "$(basename "$sums")" | sha256sum -c --quiet 2>/dev/null); then
  pass "every archive verifies against its digest"
else
  fail "an archive listed in SHA256SUMS is missing or does not match"
fi

echo
echo "== the manifest is the one in the repository =="
recorded="$(awk '$2 ~ /_manifest\.json$/ {print $1}' "$sums")"
if [[ -z "$recorded" ]]; then
  fail "SHA256SUMS records no manifest — the Registry needs one to learn the protocol"
elif [[ ! -f terraform-registry-manifest.json ]]; then
  fail "terraform-registry-manifest.json is missing from the repository"
else
  actual="$(sha256sum terraform-registry-manifest.json | cut -d' ' -f1)"
  if [[ "$recorded" = "$actual" ]]; then
    pass "manifest      matches terraform-registry-manifest.json"
  else
    fail "the recorded manifest digest is not the repository's manifest"
  fi
  grep -qE '"protocol_versions"' terraform-registry-manifest.json ||
    fail "the manifest declares no protocol_versions"
fi

echo
if [[ "$bad" -gt 0 ]]; then
  printf 'check-release-artifacts: %d problem(s) — the Registry would refuse this\n' "$bad" >&2
  exit 1
fi
printf 'check-release-artifacts: the release is shaped for ingestion\n'
