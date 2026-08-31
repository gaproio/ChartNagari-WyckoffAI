#!/usr/bin/env bash
set -euo pipefail

REMOTE="${BTC_MASTER_REMOTE:-mygithub}"
BRANCH="${BTC_MASTER_BRANCH:-wyckoff-v2}"
DAYS="${BTC_MASTER_DAYS:-3300}"
END_DATE="${BTC_MASTER_END:-2026-08-31}"
FEE_BPS="${BTC_MASTER_FEE_BPS:-10}"
SLIPPAGE_BPS="${BTC_MASTER_SLIPPAGE_BPS:-5}"
REPORT_DIR="research/btc15m"
REPORT_FILE="${REPORT_DIR}/latest.txt"
ERROR_FILE="${REPORT_DIR}/latest_error.txt"

if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
  echo "Run this command from inside the ChartNagari repository."
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

CURRENT_BRANCH="$(git branch --show-current)"
if [[ "$CURRENT_BRANCH" != "$BRANCH" ]]; then
  echo "Expected branch '$BRANCH' but currently on '$CURRENT_BRANCH'."
  echo "Switch first with: git switch $BRANCH"
  exit 1
fi

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "Git remote '$REMOTE' was not found."
  echo "Available remotes:"
  git remote -v
  exit 1
fi

# Keep code current, but refuse merges/rebases so the research state stays clean.
git pull --ff-only "$REMOTE" "$BRANCH"

echo
echo "== BTCUSDT / 15M tests =="
go test ./internal/wyckoff
go test ./...

mkdir -p "$REPORT_DIR"
TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

{
  echo "# BTCUSDT / 15M automated master report"
  echo "# generated_utc: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "# git_commit_before_report: $(git rev-parse HEAD)"
  echo "# days: $DAYS"
  echo "# end: $END_DATE"
  echo "# fee_bps_per_side: $FEE_BPS"
  echo "# slippage_bps_per_side: $SLIPPAGE_BPS"
  echo
  go run ./cmd/btc-master-validate \
    -days "$DAYS" \
    -end "$END_DATE" \
    -fee-bps "$FEE_BPS" \
    -slippage-bps "$SLIPPAGE_BPS"
} | tee "$TMP_FILE"

mv "$TMP_FILE" "$REPORT_FILE"
trap - EXIT

git add "$REPORT_FILE"
if git ls-files --error-unmatch "$ERROR_FILE" >/dev/null 2>&1; then
  git rm -f "$ERROR_FILE"
fi

if git diff --cached --quiet; then
  echo
  echo "Report is unchanged; nothing to commit."
  echo "Saved at: $REPORT_FILE"
  exit 0
fi

git commit -m "research: refresh BTCUSDT 15M master report"
git push "$REMOTE" "HEAD:$BRANCH"

echo
echo "BTCUSDT / 15M report published successfully."
echo "GitHub path: $REPORT_FILE"
echo "Autonomous loop can now consume this report."
