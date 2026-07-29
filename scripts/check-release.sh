#!/usr/bin/env bash
# Everything about a release that can be checked before anything is built.
#
# The Terraform Registry treats a published version as immutable. There is no
# amending a release: a wrong one is corrected by publishing another version
# and leaving the wrong one visible forever. So the expensive half of the
# release path — building, signing, uploading — must not start until the cheap
# half has agreed with itself.
#
# Usage: check-release.sh [version]
#   with no argument, the version is read from VERSION.
set -euo pipefail

readonly MANIFEST=terraform-registry-manifest.json
declare -i bad=0

fail() {
  printf 'FAIL  %s\n' "$1" >&2
  bad+=1
}
pass() { printf 'ok    %s\n' "$1"; }

# --- the version everything else is checked against ---

if [[ ! -f VERSION ]]; then
  echo "VERSION does not exist; run this from the repository root" >&2
  exit 2
fi
file_version="$(tr -d '[:space:]' <VERSION)"
version="${1:-$file_version}"
version="${version#v}"

echo "== version =="
if [[ "$version" = "$file_version" ]]; then
  pass "VERSION and the release agree on ${version}"
else
  # docs/standards/versioning.md calls VERSION the source of truth and says the
  # tag is cut from it. A tag that disagrees means one of the two was edited
  # and the other forgotten, and the Registry will believe the tag.
  fail "releasing ${version} but VERSION says ${file_version}"
fi

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  fail "${version} is not a semantic version"
fi

echo
echo "== the tag is new =="
# Re-tagging is the failure mode this guards. The Registry has already read the
# old tag; moving it changes what a signature covers without changing what
# anyone downloaded.
if git rev-parse -q --verify "refs/tags/v${version}" >/dev/null 2>&1; then
  head_sha="$(git rev-parse HEAD)"
  tag_sha="$(git rev-parse "v${version}^{commit}")"
  if [[ "$head_sha" = "$tag_sha" ]]; then
    pass "v${version} already points at HEAD"
  else
    fail "v${version} exists and points at ${tag_sha:0:12}, not at HEAD"
  fi
else
  pass "v${version} does not exist yet"
fi

echo
echo "== the changelog has a section for it =="
if ! grep -q "^## \[${version}\]" CHANGELOG.md; then
  # The release notes are extracted from this section. Without it the release
  # ships either nothing or, worse, whatever [Unreleased] happened to hold.
  fail "CHANGELOG.md has no '## [${version}]' section"
else
  body="$(awk -v ver="${version}" '
    /^## \[/ { if (capture) exit; if ($0 ~ "^## \\[" ver "\\]") { capture = 1; next } }
    capture { print }
  ' CHANGELOG.md | grep -c '[^[:space:]]' || true)"
  if [[ "$body" -eq 0 ]]; then
    fail "the '## [${version}]' section is empty"
  else
    pass "changelog section present, ${body} non-blank lines"
  fi
fi

echo
echo "== nothing has been added to a released section =="
# A released section describes what shipped. Writing into it after the tag
# claims the change went out when it did not, and the next release cut — which
# reads only [Unreleased] — silently drops it. Twelve entries had accumulated
# in [0.1.1] before a reviewer noticed.
#
# Additions are the fault; removals are how one is corrected, so only lines
# present now and absent at the tag count. A section that has had entries taken
# back out of it passes, which is the state this check was written in.
section_of() {
  awk -v want="$1" '
    /^## \[/ {
      if (capture) exit
      if (index($0, "## [" want "]") == 1) { capture = 1; next }
    }
    /^\[[^]]+\]:[[:space:]]*http/ { if (capture) exit }
    capture { print }
  '
}

while read -r released; do
  [[ -n "$released" ]] || continue
  tag="v${released}"
  if ! git rev-parse -q --verify "refs/tags/${tag}" >/dev/null 2>&1; then
    printf 'skip      [%s] has no tag yet\n' "$released"
    continue
  fi

  added="$(comm -13 \
    <(git show "${tag}:CHANGELOG.md" 2>/dev/null | section_of "$released" | sort -u) \
    <(section_of "$released" <CHANGELOG.md | sort -u) |
    grep -c '[^[:space:]]' || true)"

  if [[ "$added" -eq 0 ]]; then
    pass "[${released}] has gained nothing since ${tag}"
  else
    fail "[${released}] has ${added} line(s) added since ${tag}; new entries belong under [Unreleased]"
  fi
done < <(grep -oE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md | tr -d '#[] ')
echo
echo "== the manifest matches the protocol the provider serves =="
# The Registry advertises what this file says. If it says 5.0 and the binary
# speaks 6, every consumer's `terraform init` negotiates a protocol the plugin
# does not implement — and the version cannot be withdrawn.
if [[ ! -f "$MANIFEST" ]]; then
  fail "${MANIFEST} does not exist"
else
  declared="$(grep -oE '"[0-9]+\.[0-9]+"' "$MANIFEST" | tr -d '"' | tr '\n' ' ' | sed 's/ $//')"
  # terraform-plugin-framework serves protocol 6 unless ServeOpts asks for 5.
  if grep -qE 'ProtocolVersion:[[:space:]]*5' main.go; then
    served=5.0
  else
    served=6.0
  fi
  if [[ "$declared" = "$served" ]]; then
    pass "manifest declares ${declared}, and the provider serves it"
  else
    fail "manifest declares '${declared}' but the provider serves ${served}"
  fi
fi

echo
echo "== the working tree is the thing being released =="
if [[ -n "$(git status --porcelain)" ]]; then
  # goreleaser stamps the build with `git describe`, which would carry -dirty,
  # and the archive would not correspond to any commit anyone can check out.
  fail "the working tree has uncommitted changes"
  git status --porcelain | head -10 >&2
else
  pass "clean"
fi

echo
if [[ "$bad" -gt 0 ]]; then
  printf 'check-release: %d problem(s) — not releasable\n' "$bad" >&2
  exit 1
fi
printf 'check-release: v%s is releasable\n' "$version"
