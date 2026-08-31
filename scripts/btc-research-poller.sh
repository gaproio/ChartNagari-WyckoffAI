#!/usr/bin/env bash
set -euo pipefail

REMOTE="${BTC_MASTER_REMOTE:-mygithub}"
BRANCH="${BTC_MASTER_BRANCH:-wyckoff-v2}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCK_DIR="${TMPDIR:-/tmp}/chartnagari-btc15m-research.lock"

cd "$ROOT"

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  echo "BTC15M research poller: another run is active; skipping."
  exit 0
fi
trap 'rmdir "$LOCK_DIR" 2>/dev/null || true' EXIT

CURRENT_BRANCH="$(git branch --show-current)"
if [[ "$CURRENT_BRANCH" != "$BRANCH" ]]; then
  echo "BTC15M research poller: expected branch '$BRANCH', found '$CURRENT_BRANCH'; skipping."
  exit 0
fi

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "BTC15M research poller: remote '$REMOTE' not found; skipping."
  exit 0
fi

# Never touch an actively edited working tree.
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  echo "BTC15M research poller: tracked working tree is dirty; skipping for safety."
  exit 0
fi

git fetch "$REMOTE" "$BRANCH" --quiet
LOCAL_SHA="$(git rev-parse HEAD)"
REMOTE_SHA="$(git rev-parse "$REMOTE/$BRANCH")"

if [[ "$LOCAL_SHA" == "$REMOTE_SHA" ]]; then
  echo "BTC15M research poller: no new research code."
  exit 0
fi

if ! git merge-base --is-ancestor "$LOCAL_SHA" "$REMOTE_SHA"; then
  echo "BTC15M research poller: remote is not a fast-forward; manual review required."
  exit 0
fi

echo "BTC15M research poller: new remote commit detected."
git pull --ff-only "$REMOTE" "$BRANCH"

# The master runner re-checks tests, builds the report, commits it and pushes it.
BTC_MASTER_REMOTE="$REMOTE" BTC_MASTER_BRANCH="$BRANCH" bash ./scripts/btc-master-auto.sh
