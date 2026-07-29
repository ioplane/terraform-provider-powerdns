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
    # "the registry said no such digest" and "the request did not get through"
    # are different answers, and conflating them is how a rate-limited run
    # accuses a correct pin of being fabricated. The action check below learned
    # this first; the image check had not.
    if err="$(skopeo inspect "docker://${resolvable}" --format '{{.Digest}}' 2>&1 >/dev/null)"; then
      printf 'ok        %s\n' "${ref%%@*}"
      ok+=1
    elif printf '%s' "$err" | grep -qiE 'manifest unknown|not found|unknown: '; then
      printf 'NOT FOUND %s\n' "$ref" >&2
      bad+=1
    else
      printf 'UNVERIFIED %s — %s\n' "$ref" "$(printf '%s' "$err" | tail -1 | cut -c1-120)" >&2
      bad+=1
    fi
  else
    printf 'skip      %s (skopeo unavailable)\n' "${ref%%@*}"
  fi
done < <(grep -rhoE '(docker\.io|ghcr\.io|quay\.io)/[a-z0-9./_-]+:[a-zA-Z0-9._-]+(@sha256:[0-9a-f]{64})?' \
  deployments/ .github/ 2>/dev/null | sort -u)

echo
echo "== unpinned FROM in Containerfiles =="
# shellcheck disable=SC2016  # a regex, not a string meant to expand
if grep -rnE '^FROM [^$]' deployments/containers/ 2>/dev/null | grep -v '@sha256:' | grep -v '\${'; then
  echo "  the FROM above does not carry a digest" >&2
  bad+=1
else
  echo "ok        every FROM is digest-pinned or uses a pinned ARG"
fi

echo
echo "== GitHub Actions =="
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  # An action may live in a subdirectory — github/codeql-action/init,
  # anchore/sbom-action/download-syft. The SHA belongs to the repository, so
  # the first two segments are what the API is asked about; matching
  # owner/repo greedily against the whole reference instead reads
  # `codeql-action/init` as a repository and reports a valid pin as missing.
  while IFS= read -r ref; do
    [ -n "$ref" ] || continue
    action="${ref%@*}"
    sha="${ref##*@}"
    repo="$(printf '%s' "$action" | cut -d/ -f1,2)"

    if ! [[ "$sha" =~ ^[0-9a-f]{40}$ ]]; then
      printf 'FLOATING  %s — pin the commit SHA\n' "$ref" >&2
      bad+=1
      continue
    fi
    # "gh failed" and "GitHub says this commit does not exist" are different
    # answers, and treating them alike is how a rate-limited run reports two
    # dozen valid pins as fabricated. The first run of this check in CI did
    # exactly that for one action out of twenty-four.
    err=""
    for attempt in 1 2 3; do
      if err="$(gh api "repos/${repo}/commits/${sha}" --jq .sha 2>&1 >/dev/null)"; then
        err=""
        break
      fi
      case "$err" in
        *"Not Found"* | *"No commit found"*) break ;;
      esac
      sleep "$attempt"
    done

    if [ -z "$err" ]; then
      printf 'ok        %-34s %s\n' "$action" "${sha:0:12}"
      ok+=1
    elif [[ "$err" == *"Not Found"* || "$err" == *"No commit found"* ]]; then
      printf 'NOT FOUND %s\n' "$ref" >&2
      bad+=1
    else
      printf 'UNVERIFIED %s — %s\n' "$ref" "$(printf '%s' "$err" | head -1)" >&2
      bad+=1
    fi
  done < <(grep -rhoE 'uses:[[:space:]]*[A-Za-z0-9._-]+/[A-Za-z0-9._/-]+@[^[:space:]]+' .github/ 2>/dev/null |
    sed -E 's/^uses:[[:space:]]*//' | sort -u)
else
  echo "skip      gh unavailable or unauthenticated; CI runs the same check"
fi

echo
if [ "$bad" -gt 0 ]; then
  printf 'check-pins: %d verified, %d wrong\n' "$ok" "$bad" >&2
  exit 1
fi
printf 'check-pins: %d references verified\n' "$ok"
