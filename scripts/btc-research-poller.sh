#!/usr/bin/env bash
set -euo pipefail

REMOTE="${BTC_MASTER_REMOTE:-mygithub}"
BRANCH="${BTC_MASTER_BRANCH:-wyckoff-v2}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCK_DIR="${TMPDIR:-/tmp}/chartnagari-btc15m-research.lock"
REPORT_COMMIT_MSG="research: refresh BTCUSDT 15M master report"
FAIL_COMMIT_MSG="research: BTCUSDT 15M loop failure"
ERROR_FILE="research/btc15m/latest_error.txt"

cd "$ROOT"

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  echo "BTC15M research poller: another run is active; skipping."
  exit 0
fi
RUN_LOG="$(mktemp)"
cleanup() {
  rm -f "$RUN_LOG"
  rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT

CURRENT_BRANCH="$(git branch --show-current)"
if [[ "$CURRENT_BRANCH" != "$BRANCH" ]]; then
  echo "BTC15M research poller: expected branch '$BRANCH', found '$CURRENT_BRANCH'; skipping."
  exit 0
fi

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "BTC15M research poller: remote '$REMOTE' not found; skipping."
  exit 0
fi

# Never touch an actively edited tracked working tree.
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  echo "BTC15M research poller: tracked working tree is dirty; skipping for safety."
  exit 0
fi

git fetch "$REMOTE" "$BRANCH" --quiet
LOCAL_SHA="$(git rev-parse HEAD)"
REMOTE_SHA="$(git rev-parse "$REMOTE/$BRANCH")"

if [[ "$LOCAL_SHA" != "$REMOTE_SHA" ]]; then
  if ! git merge-base --is-ancestor "$LOCAL_SHA" "$REMOTE_SHA"; then
    echo "BTC15M research poller: remote is not a fast-forward; manual review required."
    exit 0
  fi
  echo "BTC15M research poller: new remote commit detected."
  git pull --ff-only "$REMOTE" "$BRANCH"
fi

TIP_MSG="$(git log -1 --pretty=%s)"
if [[ "$TIP_MSG" == "$REPORT_COMMIT_MSG" ]]; then
  echo "BTC15M research poller: latest research code already has a published report."
  exit 0
fi
if [[ "$TIP_MSG" == "$FAIL_COMMIT_MSG" ]]; then
  echo "BTC15M research poller: last run failed and is waiting for assistant repair."
  exit 0
fi

# Any other tip means research code is waiting for a fresh tested report.
echo "BTC15M research poller: research code is waiting for a report; running master study."
set +e
BTC_MASTER_REMOTE="$REMOTE" BTC_MASTER_BRANCH="$BRANCH" bash ./scripts/btc-master-auto.sh 2>&1 | tee "$RUN_LOG"
STATUS=${PIPESTATUS[0]}
set -e

if [[ "$STATUS" -eq 0 ]]; then
  exit 0
fi

mkdir -p "$(dirname "$ERROR_FILE")"
{
  echo "# BTCUSDT / 15M autonomous loop failure"
  echo "# generated_utc: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "# source_commit: $(git rev-parse HEAD)"
  echo "# exit_status: $STATUS"
  echo
  cat "$RUN_LOG"
} > "$ERROR_FILE"

git add "$ERROR_FILE"
git commit -m "$FAIL_COMMIT_MSG"
git push "$REMOTE" "HEAD:$BRANCH"

echo "BTC15M research poller: failure report published for assistant repair."
exit "$STATUS"
