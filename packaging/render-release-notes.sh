#!/usr/bin/env bash
# render-release-notes.sh - Compile hybrid GitHub Release notes from CHANGELOG.md and Git DAG history.
#
# Combines curated Keep-a-Changelog capability highlights (`CHANGELOG.md`), automatic parent `vX.Y.0`
# milestone rollups on patch releases (`vX.Y.Z` where `Z > 0`), and the chronological SHA-linked
# `[INT-*]` / `[SUB-*]` commit ledger (`## Changelog`) into a single deterministic release document.

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
    flag { print }
  ' CHANGELOG.md | sed '/^---$/d'
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

PRIMARY_NOTES="$(extract_section "$TAG" | trim_markdown)"
if [ -z "$PRIMARY_NOTES" ]; then
  echo "::error::CHANGELOG.md is missing a non-empty release section for $TAG" >&2
  exit 1
fi

printf '%s\n' "$PRIMARY_NOTES"

if printf '%s\n' "$TAG" | grep -qE '^v[0-9]+\.[0-9]+\.[1-9][0-9]*$'; then
  MINOR_BASE="${TAG%.*}.0"
  MINOR_NOTES="$(extract_section "$MINOR_BASE" | trim_markdown)"
  if [ -n "$MINOR_NOTES" ]; then
    printf '\n---\n\n## Included `%s` Milestone Features\n\n%s\n' "$MINOR_BASE" "$MINOR_NOTES"
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

if [ -n "$COMMIT_LEDGER" ]; then
  printf '\n---\n\n## Changelog\n\n%s\n' "$COMMIT_LEDGER"
  if [ -n "$PREV_TAG" ]; then
    printf '\n**Full Changelog**: https://github.com/%s/compare/%s...%s\n' "$REPO_SLUG" "$PREV_TAG" "$TAG"
  fi
fi
