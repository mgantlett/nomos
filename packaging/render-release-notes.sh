#!/usr/bin/env bash
# render-release-notes.sh - Compile hybrid GitHub Release notes from CHANGELOG.md and Git DAG history.
#
# Leads with the 1-line release summary (`> ...`) and the SHA-linked `[INT-*]` / `[SUB-*]` commit
# ledger (`## Changelog`) grouped by the 6 canonical Open-Core areas, followed by collapsible
# `<details>` disclosures for curated `CHANGELOG.md` architecture notes and automatic parent
# `vX.Y.0` milestone rollups on patch releases.

set -euo pipefail

TAG="${1:-}"
if [ -z "$TAG" ]; then
  echo "::error::Usage: packaging/render-release-notes.sh <vX.Y.Z>" >&2
  exit 1
fi

if ! printf '%s\n' "$TAG" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "::error::Invalid SemVer release tag '$TAG' (expected vX.Y.Z)" >&2
  exit 1
fi

REPO_SLUG="${GITHUB_REPOSITORY:-mgantlett/nomos}"

extract_section() {
  awk -v tag="[$1]" '
    index($0, "## " tag) == 1 { flag=1; next }
    flag && /^## \[/ { exit }
    flag && $0 != "---" { print }
  ' CHANGELOG.md
}

trim_markdown() {
  awk '
    NF { found=1 }
    found { lines[++n] = $0 }
    END {
      while (n > 0 && lines[n] ~ /^[[:space:]]*$/) n--
      for (i = 1; i <= n; i++) print lines[i]
    }
  '
}

group_commit_ledger() {
  awk '
    NF == 0 { next }
    {
      rest = $0
      sub(/^\* [^ ]+ /, "", rest)
      bucket = 0
      if (rest ~ /^\[INT-[0-9]+\]/) {
        bucket = 1
      } else {
        conv = rest
        sub(/^\[[A-Z]+-[0-9]+\][[:space:]]*/, "", conv)
        scope = ""
        if (conv ~ /^[A-Za-z0-9_-]+\([^)]+\)!?:/) {
          lparen = index(conv, "(")
          rparen = index(conv, ")")
          if (lparen > 0 && rparen > lparen) {
            raw_scope = tolower(substr(conv, lparen + 1, rparen - lparen - 1))
            sub(/[,\/[:space:]].*$/, "", raw_scope)
            scope = raw_scope
          }
        }
        if (scope ~ /^(intent|spec|specs)$/) {
          bucket = 1
        } else if (scope ~ /^(cli|install|update|handshake|hook|hooks|ui)$/) {
          bucket = 2
        } else if (scope ~ /^(verify|ncte|treesitter|capbac|physics|ast)$/) {
          bucket = 3
        } else if (scope ~ /^(config|workspace|storage|constants|types)$/) {
          bucket = 4
        } else if (scope ~ /^(task|mem|session|telemetry|learn)$/) {
          bucket = 5
        } else {
          bucket = 6
        }
      }
      if (count[bucket] == 0) {
        items[bucket] = $0
      } else {
        items[bucket] = items[bucket] "\n" $0
      }
      count[bucket]++
    }
    END {
      headers[1] = "### Intent & Specifications"
      headers[2] = "### CLI, Distribution & Hooks"
      headers[3] = "### Verification & Quality Gates"
      headers[4] = "### Configuration & Workspace"
      headers[5] = "### Task, Memory & Session"
      headers[6] = "### Release & Packaging"
      printed = 0
      for (b = 1; b <= 6; b++) {
        if (count[b] > 0) {
          if (printed) printf "\n"
          printf "%s\n%s\n", headers[b], items[b]
          printed = 1
        }
      }
    }
  '
}

PRIMARY_NOTES="$(extract_section "$TAG" | trim_markdown)"
if [ -z "$PRIMARY_NOTES" ]; then
  echo "::error::CHANGELOG.md is missing a non-empty release section for $TAG" >&2
  exit 1
fi

FIRST_LINE="${PRIMARY_NOTES%%$'\n'*}"
TAGLINE=""
DETAIL_BODY="$PRIMARY_NOTES"
if [[ "$FIRST_LINE" == "> "* ]]; then
  TAGLINE="$FIRST_LINE"
  if [[ "$PRIMARY_NOTES" == *$'\n'* ]]; then
    DETAIL_BODY="$(printf '%s\n' "${PRIMARY_NOTES#*$'\n'}" | trim_markdown)"
  else
    DETAIL_BODY=""
  fi
fi

TARGET_REF="HEAD"
TAG_EXISTS=0
if git rev-parse --verify --quiet "${TAG}^{commit}" >/dev/null; then
  TARGET_REF="${TAG}^{commit}"
  TAG_EXISTS=1
fi

PREV_TAG=""
LAST_SEEN=""
while IFS= read -r candidate; do
  [ -z "$candidate" ] && continue
  if ! printf '%s\n' "$candidate" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    continue
  fi
  if [ "$candidate" = "$TAG" ]; then
    PREV_TAG="$LAST_SEEN"
    break
  fi
  LAST_SEEN="$candidate"
done < <(git tag -l 'v*' --sort=v:refname)

if [ "$TAG_EXISTS" -eq 0 ] && [ -z "$PREV_TAG" ]; then
  PREV_TAG="$LAST_SEEN"
fi

if [ -n "$PREV_TAG" ]; then
  COMMIT_LEDGER="$(git log "${PREV_TAG}..${TARGET_REF}" --reverse --format='* %H %s')"
else
  COMMIT_LEDGER="$(git log "${TARGET_REF}" --reverse --format='* %H %s')"
fi

PRINTED=0
emit_block() {
  if [ "$PRINTED" -eq 1 ]; then
    printf '\n'
  fi
  printf '%s\n' "$1"
  PRINTED=1
}

if [ -n "$TAGLINE" ]; then
  emit_block "$TAGLINE"
fi

if [ -n "$COMMIT_LEDGER" ]; then
  GROUPED_LEDGER="$(printf '%s\n' "$COMMIT_LEDGER" | group_commit_ledger)"
  LEDGER_SECTION="$(printf '## Changelog\n\n%s' "$GROUPED_LEDGER")"
  if [ -n "$PREV_TAG" ]; then
    LEDGER_SECTION="$(printf '%s\n\n**Full Changelog**: https://github.com/%s/compare/%s...%s' "$LEDGER_SECTION" "$REPO_SLUG" "$PREV_TAG" "$TAG")"
  fi
  emit_block "$LEDGER_SECTION"
fi

if [ -n "$DETAIL_BODY" ]; then
  DETAILS_BLOCK="$(printf '<details>\n<summary><strong>Release Notes &amp; Architecture Details</strong></summary>\n\n%s\n\n</details>' "$DETAIL_BODY")"
  emit_block "$DETAILS_BLOCK"
fi

if printf '%s\n' "$TAG" | grep -qE '^v[0-9]+\.[0-9]+\.[1-9][0-9]*$'; then
  MINOR_BASE="${TAG%.*}.0"
  MINOR_NOTES="$(extract_section "$MINOR_BASE" | trim_markdown)"
  if [ -n "$MINOR_NOTES" ]; then
    ROLLUP_BLOCK="$(printf '<details>\n<summary><strong>Included <code>%s</code> Milestone Features</strong></summary>\n\n%s\n\n</details>' "$MINOR_BASE" "$MINOR_NOTES")"
    emit_block "$ROLLUP_BLOCK"
  fi
fi
