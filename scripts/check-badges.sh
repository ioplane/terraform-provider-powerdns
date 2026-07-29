#!/usr/bin/env bash
# Verifies that every shieldcn badge URL in the documentation resolves, and
# that every mermaid block is at least well-formed enough to render.
#
# A badge pointing at a non-existent endpoint renders as a broken image on the
# front page of the project. Nobody catches that in review; everybody sees it
# afterwards. Same argument as scripts/check-pins.sh: a rule nobody checks is a
# preference.
set -euo pipefail

declare -i badges_ok=0 badges_bad=0 mermaid_ok=0 mermaid_bad=0

echo "== shieldcn badges =="
# Fenced blocks are excluded: they hold templates and examples, and shieldcn
# renders any static label, so a placeholder would pass and prove nothing.
mapfile -t urls < <(
  find . -name '*.md' -not -path './.git/*' -not -path './node_modules/*' -print0 \
  | xargs -0 awk '/^```/{f=!f; next} !f' 2>/dev/null \
  | grep -oE 'https://shieldcn\.dev/[^")[:space:]]+' \
  | sed 's/&amp;/\&/g' | sort -u
)

if [ ${#urls[@]} -eq 0 ]; then
  echo "no badges found"
else
  for url in "${urls[@]}"; do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 "$url" || echo 000)
    if [ "$code" != "200" ]; then
      printf 'FAIL  %s  (HTTP %s)\n' "${url#https://shieldcn.dev}" "$code" >&2
      badges_bad+=1
      continue
    fi

    # A 200 is not enough. shieldcn renders an error card rather than failing,
    # so github/ci for a repository with no GitHub Actions CI answers 200 and
    # displays "not found". Ask the .json for the resolved value.
    case "$url" in
      *shieldcn.dev/github/*)
        json_url="${url%%\?*}"
        json_url="${json_url%.svg}.json"
        if curl -s --max-time 15 "$json_url" | grep -q '"error"'; then
          printf 'FAIL  %s  (endpoint resolves to an error)\n' "${url#https://shieldcn.dev}" >&2
          badges_bad+=1
          continue
        fi
        ;;
    esac

    printf 'ok    %s\n' "${url#https://shieldcn.dev}"
    badges_ok+=1
  done
fi

echo
echo "== mermaid blocks =="
# Structural checks only: a full parse would need a browser. These catch the
# mistakes that actually happen — unbalanced fences, unquoted labels containing
# punctuation that terminates the node early.
while IFS= read -r file; do
  opens=$(grep -c '^```mermaid' "$file" || true)
  [ "$opens" -eq 0 ] && continue
  fences=$(grep -c '^```' "$file" || true)
  if [ $((fences % 2)) -ne 0 ]; then
    printf 'FAIL  %s: unbalanced code fences\n' "$file" >&2
    mermaid_bad+=1
    continue
  fi
  # A node label containing a colon, bracket or slash must be quoted.
  if awk '/^```mermaid/{m=1;next} /^```/{m=0} m' "$file" \
       | grep -nE '^\s*[A-Za-z0-9_]+\[[^"]*[:/()][^"]*\]' >/dev/null; then
    printf 'FAIL  %s: unquoted node label containing punctuation\n' "$file" >&2
    mermaid_bad+=1
    continue
  fi
  printf 'ok    %s (%s block(s))\n' "$file" "$opens"
  mermaid_ok+=1
done < <(find . -name '*.md' -not -path './node_modules/*' -not -path './.git/*')

echo
echo "== the plan's task counter =="
# The audit of 2026-07-29 found this badge reading 67 against 62 tasks actually
# marked done, because it was hand-incremented every sprint. Recomputing it once
# fixes the number; only this stops it happening again. A counter nobody
# recomputes is a guess with a green background.
declare -i counter_bad=0
if [ -f docs/plan.md ]; then
  claimed="$(grep -oE 'badge/tasks_done-[0-9]+-' docs/plan.md | head -1 | grep -oE '[0-9]+' || true)"
  actual="$(grep -cE '^\| S[0-9]+-[0-9]+ \|.*`\[x\]`' docs/plan.md || true)"
  if [ -z "$claimed" ]; then
    echo "FAIL  docs/plan.md has no tasks_done badge" >&2
    counter_bad=1
  elif [ "$claimed" != "$actual" ]; then
    printf 'FAIL  badge claims %s done, the tables show %s\n' "$claimed" "$actual" >&2
    counter_bad=1
  else
    printf 'ok    %s tasks marked done, badge agrees\n' "$actual"
  fi
fi

echo
if [ $((badges_bad + mermaid_bad + counter_bad)) -gt 0 ]; then
  printf 'check-badges: %d badges ok, %d bad; %d files with mermaid ok, %d bad; counter %s\n' \
    "$badges_ok" "$badges_bad" "$mermaid_ok" "$mermaid_bad" \
    "$([ "$counter_bad" -eq 0 ] && echo ok || echo wrong)" >&2
  exit 1
fi
printf 'check-badges: %d badges verified, %d files with mermaid verified, counter agrees\n' \
  "$badges_ok" "$mermaid_ok"
