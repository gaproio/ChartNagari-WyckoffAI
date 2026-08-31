#!/usr/bin/env bash
set -euo pipefail

LABEL="com.chartnagari.btc15m-research"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLIST="$HOME/Library/LaunchAgents/${LABEL}.plist"
LOG_DIR="$HOME/Library/Logs/ChartNagari"
LOG_FILE="$LOG_DIR/btc15m-research.log"
ERR_FILE="$LOG_DIR/btc15m-research.err.log"
UID_NUM="$(id -u)"

uninstall() {
  launchctl bootout "gui/$UID_NUM" "$PLIST" 2>/dev/null || true
  rm -f "$PLIST"
  echo "BTCUSDT/15M background research loop removed."
}

if [[ "${1:-}" == "uninstall" ]]; then
  uninstall
  exit 0
fi

mkdir -p "$HOME/Library/LaunchAgents" "$LOG_DIR"

cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>$ROOT/scripts/btc-research-poller.sh</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$ROOT</string>
  <key>StartInterval</key>
  <integer>1200</integer>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>$LOG_FILE</string>
  <key>StandardErrorPath</key>
  <string>$ERR_FILE</string>
</dict>
</plist>
EOF

plutil -lint "$PLIST" >/dev/null
launchctl bootout "gui/$UID_NUM" "$PLIST" 2>/dev/null || true
launchctl bootstrap "gui/$UID_NUM" "$PLIST"
launchctl enable "gui/$UID_NUM/$LABEL"
launchctl kickstart -k "gui/$UID_NUM/$LABEL" >/dev/null 2>&1 || true

echo "BTCUSDT/15M background research loop installed."
echo "Checks for new research code every 20 minutes."
echo "Logs: $LOG_FILE"
echo "Errors: $ERR_FILE"
echo "To remove it later: make btc-research-loop-uninstall"
