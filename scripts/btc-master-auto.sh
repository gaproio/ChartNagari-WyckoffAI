#!/usr/bin/env bash
set -euo pipefail

REMOTE="${BTC_MASTER_REMOTE:-mygithub}"
BRANCH="${BTC_MASTER_BRANCH:-wyckoff-v2}"
DAYS="${BTC_MASTER_DAYS:-3300}"
END_DATE="${BTC_MASTER_END:-2026-08-31}"
FEE_BPS="${BTC_MASTER_FEE_BPS:-10}"
SLIPPAGE_BPS="${BTC_MASTER_SLIPPAGE_BPS:-5}"
NOTIFY_CHATGPT="${BTC_MASTER_NOTIFY_CHATGPT:-auto}"
REPORT_DIR="research/btc15m"
REPORT_FILE="${REPORT_DIR}/latest.txt"
ERROR_FILE="${REPORT_DIR}/latest_error.txt"

# launchd uses a minimal PATH, so resolve Go explicitly before running tests.
# BTC_MASTER_GO can override this when Go is installed in a non-standard path.
GO_BIN="${BTC_MASTER_GO:-}"
if [[ -z "$GO_BIN" ]]; then
  if command -v go >/dev/null 2>&1; then
    GO_BIN="$(command -v go)"
  else
    for candidate in /usr/local/bin/go /usr/local/go/bin/go /opt/homebrew/bin/go; do
      if [[ -x "$candidate" ]]; then
        GO_BIN="$candidate"
        break
      fi
    done
  fi
fi
if [[ -z "$GO_BIN" || ! -x "$GO_BIN" ]]; then
  echo "Go executable not found. Set BTC_MASTER_GO to the full path of go."
  exit 127
fi

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
# Important: this bash process may continue executing the script body loaded
# before the pull. Changes to this runner itself therefore become active on the
# following invocation; same-run diagnostics should live in Go helpers instead.
git pull --ff-only "$REMOTE" "$BRANCH"

echo
echo "== BTCUSDT / 15M tests =="
"$GO_BIN" test ./internal/wyckoff
"$GO_BIN" test ./...

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
  "$GO_BIN" run ./cmd/btc-master-validate \
    -days "$DAYS" \
    -end "$END_DATE" \
    -fee-bps "$FEE_BPS" \
    -slippage-bps "$SLIPPAGE_BPS"
} | tee "$TMP_FILE"

mv "$TMP_FILE" "$REPORT_FILE"
trap - EXIT

# Sequence/tail diagnostic parses the exact chronological trade lines already
# printed above, so it adds no second market-data fetch and changes no rules.
"$GO_BIN" run ./cmd/btc-sequence-report -file "$REPORT_FILE" | tee -a "$REPORT_FILE"

# Rolling stability uses the same chronological frozen trade lines already in
# the report. A fixed five-trade window is descriptive only and changes no rule.
awk '
  substr($1,5,1)=="-" && substr($1,8,1)=="-" && length($1)==10 && $3=="|" {
    for (j=1; j<=NF; j++) {
      if ($j=="net" && j<NF) {
        v=$(j+1)
        sub(/R$/, "", v)
        n++
        ts[n]=$1 " " $2
        r[n]=v+0
        break
      }
    }
  }
  END {
    print ""
    print "BTC 15M rolling 5-trade stability diagnostic (DESCRIPTIVE; frozen rules unchanged):"
    print "Chronological overlapping five-trade windows. This measures local edge stability only; it is not a trade filter."
    w=5
    if (n<w) {
      printf("insufficient trades: n=%d, need %d\n", n, w)
      exit
    }
    windows=0; pos=0; neg=0; flat=0
    for (s=1; s+w-1<=n; s++) {
      e=s+w-1
      sum=0
      for (i=s; i<=e; i++) sum+=r[i]
      avg=sum/w
      windows++
      if (avg>0) pos++; else if (avg<0) neg++; else flat++
      if (windows==1 || avg<minAvg) { minAvg=avg; minStart=s; minEnd=e }
      if (windows==1 || avg>maxAvg) { maxAvg=avg; maxStart=s; maxEnd=e }
      printf("window %02d-%02d | %s -> %s | total %+.3fR avg %+.3fR\n", s, e, ts[s], ts[e], sum, avg)
    }
    printf("windows %d | positive/negative/flat %d/%d/%d | worst avg %+.3fR trades %02d-%02d | best avg %+.3fR trades %02d-%02d\n",
      windows, pos, neg, flat, minAvg, minStart, minEnd, maxAvg, maxStart, maxEnd)
  }
' "$REPORT_FILE" | tee -a "$REPORT_FILE"

git add "$REPORT_FILE"
if git ls-files --error-unmatch "$ERROR_FILE" >/dev/null 2>&1; then
  git rm -f "$ERROR_FILE"
fi

if git diff --cached --quiet; then
  echo
  echo "Report is unchanged; nothing to commit."
  echo "Saved at: $REPORT_FILE"
else
  git commit -m "research: refresh BTCUSDT 15M master report"
  git push "$REMOTE" "HEAD:$BRANCH"

  echo
  echo "BTCUSDT / 15M report published successfully."
  echo "GitHub path: $REPORT_FILE"
  echo "Autonomous loop can now consume this report."
fi

# Fast mode: when this script is run manually in an interactive terminal,
# best-effort notify the front ChatGPT Brave/Chrome tab by sending "done".
# Background launchd/poller runs are piped/non-interactive, so auto mode skips.
SHOULD_NOTIFY=0
NOTIFY_NORMALIZED="$(printf '%s' "$NOTIFY_CHATGPT" | tr '[:upper:]' '[:lower:]')"
case "$NOTIFY_NORMALIZED" in
  1|true|yes|on)
    SHOULD_NOTIFY=1
    ;;
  0|false|no|off)
    SHOULD_NOTIFY=0
    ;;
  auto)
    if [[ -t 1 ]]; then SHOULD_NOTIFY=1; fi
    ;;
  *)
    echo "Unknown BTC_MASTER_NOTIFY_CHATGPT='$NOTIFY_CHATGPT'; skipping ChatGPT notification."
    ;;
esac

if [[ "$SHOULD_NOTIFY" -eq 1 ]]; then
  bash ./scripts/chatgpt-done-macos.sh || true
fi
