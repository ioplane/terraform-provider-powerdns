#!/usr/bin/env bash
# Asserts that every tool version a workflow names matches the version the dev
# image is built with.
#
# The gate runs inside a podman-compose dev container. A hosted runner cannot
# cheaply reproduce that — building the image costs more per job than the job
# — so the workflows install the same tools themselves. The toolchain now
# exists in two places, and two places drift.
#
# The drift is the whole risk. A linter at v2.12.2 locally and v2.13 in CI
# disagrees about a finding, and the argument that follows is about which
# machine is right rather than about the code. So Containerfile.dev holds the
# versions, a workflow line that names one carries `# pin: <ARG>`, and this
# refuses the mismatch.
#
# Two directions are checked, because only one of them is the obvious one:
#
#   * a marked line must contain its ARG's value  — catches a bumped version
#   * every required ARG must be marked somewhere — catches a deleted marker,
#     which otherwise turns the check off silently and reads as passing
set -euo pipefail

readonly CONTAINERFILE=deployments/containers/Containerfile.dev
readonly WORKFLOW_DIR=.github/workflows
# pyproject declares three of the same versions. It was missed once, and the
# drift only surfaced when a workflow installed a different podman-py.
readonly PINNED_FILES=("$WORKFLOW_DIR" pyproject.toml)

# The tools CI is expected to install. An ARG absent from this list is one the
# dev image needs and CI does not — Task, OpenTofu and Terragrunt are for
# developers, and requiring a marker for them would force a fake reference.
readonly REQUIRED=(
  GO_IMAGE
  GOLANGCI_LINT_VERSION
  GOVULNCHECK_VERSION
  OSV_SCANNER_VERSION
  TFPLUGINDOCS_VERSION
  GORELEASER_VERSION
  SYFT_VERSION
  GOTESTSUM_VERSION
  TERRAFORM_VERSION
  NODE_MAJOR
  MARKDOWNLINT_VERSION
  CSPELL_VERSION
  COMMITLINT_VERSION
  COMMITLINT_CONFIG_VERSION
  UV_VERSION
  RUFF_VERSION
  TY_VERSION
  YAMLLINT_VERSION
  SEMGREP_VERSION
  SHELLCHECK_VERSION
  SHFMT_VERSION
  HADOLINT_VERSION
  ACTIONLINT_VERSION
  ZIZMOR_VERSION
  PODMAN_PY_VERSION
)

declare -A expected=()
declare -A seen=()
declare -i bad=0

# Read the ARG defaults. GO_IMAGE carries a tag and a digest; the digest is the
# part that pins, and it is the part a workflow's `container:` reference has in
# common with a Containerfile's FROM.
while IFS= read -r line; do
  [[ "$line" =~ ^ARG[[:space:]]+([A-Z0-9_]+)=(.+)$ ]] || continue
  name="${BASH_REMATCH[1]}"
  value="${BASH_REMATCH[2]}"
  [[ "$value" == *@* ]] && value="${value#*@}"
  expected["$name"]="$value"
done <"$CONTAINERFILE"

echo "== workflow pins vs ${CONTAINERFILE} =="

while IFS= read -r hit; do
  file="${hit%%:*}"
  rest="${hit#*:}"
  lineno="${rest%%:*}"
  text="${rest#*:}"

  [[ "$text" =~ \#[[:space:]]*pin:[[:space:]]*([A-Z0-9_]+) ]] || continue
  name="${BASH_REMATCH[1]}"

  if [[ -z "${expected[$name]+set}" ]]; then
    printf 'UNKNOWN   %s:%s names %s, which %s does not define\n' \
      "$file" "$lineno" "$name" "$CONTAINERFILE" >&2
    bad+=1
    continue
  fi

  # Match against the line without its marker: a comment naming the ARG must
  # not be able to satisfy the check on its own.
  code="${text%%#*}"
  if [[ "$code" == *"${expected[$name]}"* ]]; then
    printf 'ok        %-28s %s\n' "$name" "${expected[$name]}"
    seen["$name"]=1
  else
    printf 'MISMATCH  %s:%s expects %s=%s\n' \
      "$file" "$lineno" "$name" "${expected[$name]}" >&2
    printf '          line reads: %s\n' "$(printf '%s' "$code" | sed 's/^[[:space:]]*//')" >&2
    bad+=1
  fi
done < <(grep -rn '# pin:' "${PINNED_FILES[@]}" 2>/dev/null || true)

echo
echo "== every CI tool is pinned somewhere =="
for name in "${REQUIRED[@]}"; do
  if [[ -z "${expected[$name]+set}" ]]; then
    printf 'MISSING   %s is required but %s does not define it\n' "$name" "$CONTAINERFILE" >&2
    bad+=1
  elif [[ -z "${seen[$name]+set}" ]]; then
    printf 'UNPINNED  %s is not referenced anywhere\n' "$name" >&2
    bad+=1
  else
    printf 'ok        %s\n' "$name"
  fi
done

echo
if [[ "$bad" -gt 0 ]]; then
  echo "${bad} problem(s) — the dev image and CI would run different tools" >&2
  exit 1
fi
echo "CI and the dev image agree on every pinned tool"
