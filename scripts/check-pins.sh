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

# Every manifest media type a registry might answer with. A digest is only
# retrievable if the client asks for the type it was published as, and asking
# for too few is how the same pin reads as present on one machine and missing
# on another.
readonly MANIFEST_ACCEPT='application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json'

# Ask the registry over HTTP rather than through a container client.
#
# skopeo consults local storage, mirrors and caches, so it happily resolved a
# digest that quay.io answers 404 for — the pin looked verified here and failed
# in CI, and the checker sided with the machine that was wrong. The registry is
# the authority on whether a digest exists; ask it directly.
#
# Prints the HTTP status, or 000 if the request never completed.
manifest_status() {
  local ref="$1"
  local registry="${ref%%/*}" rest="${ref#*/}"
  local repo="${rest%@*}" digest="${rest#*@}"
  local host="$registry"
  [[ "$registry" = "docker.io" ]] && host="registry-1.docker.io"
  local url="https://${host}/v2/${repo}/manifests/${digest}"

  local head code
  head="$(curl -sI --max-time 20 -H "Accept: ${MANIFEST_ACCEPT}" "$url" 2>/dev/null)" || {
    echo "000"
    return
  }
  code="$(printf '%s' "$head" | head -1 | awk '{print $2}')"

  # Anonymous pull: the registry says who to ask for a token and for what.
  if [[ "$code" = "401" ]]; then
    local auth realm service token
    auth="$(printf '%s' "$head" | grep -i '^www-authenticate:' | tr -d '\r')"
    realm="$(printf '%s' "$auth" | grep -oE 'realm="[^"]+"' | cut -d'"' -f2)"
    service="$(printf '%s' "$auth" | grep -oE 'service="[^"]+"' | cut -d'"' -f2)"
    [[ -n "$realm" ]] || {
      echo "401"
      return
    }
    token="$(curl -s --max-time 20 "${realm}?service=${service}&scope=repository:${repo}:pull" 2>/dev/null |
      python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("token") or d.get("access_token",""))' 2>/dev/null)"
    code="$(curl -sI --max-time 20 -o /dev/null -w '%{http_code}' \
      -H "Authorization: Bearer ${token}" -H "Accept: ${MANIFEST_ACCEPT}" "$url" 2>/dev/null)"
  fi
  echo "${code:-000}"
}

echo "== container images =="
# Every image reference in compose, CI and Containerfiles.
while IFS= read -r ref; do
  [[ -n "$ref" ]] || continue
  if ! [[ "$ref" =~ @sha256:[0-9a-f]{64}$ ]]; then
    printf 'FLOATING  %s\n' "$ref" >&2
    bad+=1
    continue
  fi
  # A reference may carry a tag and a digest for human legibility. The digest
  # is the part that pins, so the tag is dropped before asking.
  resolvable="$(printf '%s' "$ref" | sed -E 's/:[^:@/]+@/@/')"

  status="$(manifest_status "$resolvable")"
  case "$status" in
    200)
      printf 'ok        %s\n' "${ref%%@*}"
      ok+=1
      ;;
    404)
      printf 'NOT FOUND %s\n' "$ref" >&2
      bad+=1
      ;;
    *)
      printf 'UNVERIFIED %s — registry answered %s\n' "$ref" "$status" >&2
      bad+=1
      ;;
  esac
done < <(grep -rhoE '(docker\.io|ghcr\.io|quay\.io|codeberg\.org)/[a-z0-9./_-]+:[a-zA-Z0-9._-]+(@sha256:[0-9a-f]{64})?' \
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
    [[ -n "$ref" ]] || continue
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

    if [[ -z "$err" ]]; then
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
if [[ "$bad" -gt 0 ]]; then
  printf 'check-pins: %d verified, %d wrong\n' "$ok" "$bad" >&2
  exit 1
fi
printf 'check-pins: %d references verified\n' "$ok"
