#!/usr/bin/env bash
# Verifies that everything referenced by hash actually resolves, and that
# nothing floats.
#
# Two classes are checked:
#   * container images  — must carry @sha256:<64 hex>, and it must exist
#   * GitHub Actions    — must carry @<40 hex>, and it must resolve
#
# A fabricated digest or SHA is syntactically valid: it parses, reads as
# correct, survives review, and fails only when something tries to pull it.
# That is why this is a gate and not a convention.
set -euo pipefail

readonly IMAGE_RE='(image|FROM|GO_IMAGE|_IMAGE):?[[:space:]]*=?[[:space:]]*"?([a-z0-9./:_-]+)@(sha256:[0-9a-f]{64})'
declare -i ok=0 bad=0

echo "== container images =="
# Every image reference in compose, CI and Containerfiles.
while IFS= read -r ref; do
  [ -n "$ref" ] || continue
  if ! [[ "$ref" =~ @sha256:[0-9a-f]{64}$ ]]; then
    printf 'FLOATING  %s\n' "$ref" >&2
    bad+=1
    continue
  fi
  if command -v skopeo >/dev/null 2>&1; then
    # A reference may carry a tag and a digest for human legibility, but
    # skopeo accepts only one. Strip the tag and resolve by digest, which is
    # the part that actually pins.
    resolvable="${ref%%:*}"
    [[ "$ref" == */*:*@* ]] && resolvable="$(printf '%s' "$ref" | sed -E 's/:[^:@/]+@/@/')"
    if skopeo inspect "docker://${resolvable}" --format '{{.Digest}}' >/dev/null 2>&1; then
      printf 'ok        %s\n' "${ref%%@*}"
      ok+=1
    else
      printf 'NOT FOUND %s\n' "$ref" >&2
      bad+=1
    fi
  else
    printf 'skip      %s (skopeo unavailable)\n' "${ref%%@*}"
  fi
done < <(grep -rhoE '(docker\.io|ghcr\.io|quay\.io)/[a-z0-9./_-]+:[a-zA-Z0-9._-]+(@sha256:[0-9a-f]{64})?' \
           deployments/ .gitlab-ci.yml .github/ 2>/dev/null | sort -u)

echo
echo "== unpinned FROM in Containerfiles =="
if grep -rnE '^FROM [^$]' deployments/containers/ 2>/dev/null | grep -v '@sha256:' | grep -v '\${'; then
  echo "  the FROM above does not carry a digest" >&2
  bad+=1
else
  echo "ok        every FROM is digest-pinned or uses a pinned ARG"
fi

echo
echo "== GitHub Actions =="
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  while IFS='@' read -r repo sha; do
    [ -n "${sha:-}" ] || continue
    if gh api "repos/${repo}/commits/${sha}" --jq .sha >/dev/null 2>&1; then
      printf 'ok        %-34s %s\n' "$repo" "${sha:0:12}"
      ok+=1
    else
      printf 'NOT FOUND %s@%s\n' "$repo" "$sha" >&2
      bad+=1
    fi
  done < <(grep -rhoE '[A-Za-z0-9._-]+/[A-Za-z0-9._-]+@[0-9a-f]{40}' .github/ 2>/dev/null | sort -u)

  if grep -rhoE 'uses:[[:space:]]*[A-Za-z0-9._-]+/[A-Za-z0-9._-]+@[^ ]+' .github/ 2>/dev/null \
     | grep -vE '@[0-9a-f]{40}'; then
    echo "  the action above floats; pin the commit SHA" >&2
    bad+=1
  fi
else
  echo "skip      gh unavailable or unauthenticated; CI runs the same check"
fi

echo
if [ "$bad" -gt 0 ]; then
  printf 'check-pins: %d verified, %d wrong\n' "$ok" "$bad" >&2
  exit 1
fi
printf 'check-pins: %d references verified\n' "$ok"
