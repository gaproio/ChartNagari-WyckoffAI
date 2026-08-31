#!/usr/bin/env bash
set -u

# Best-effort fast-mode helper for macOS Chromium browsers.
# Brave Browser is preferred because that is the current research browser;
# Google Chrome is supported as a fallback. It intentionally does nothing
# unless the FRONT browser tab is chatgpt.com. Notification failure must never
# fail a BTC research run.

if [[ "$(uname -s)" != "Darwin" ]]; then
  exit 0
fi

if ! command -v osascript >/dev/null 2>&1; then
  exit 0
fi

BROWSER="${CHATGPT_BROWSER:-}"
if [[ -z "$BROWSER" ]]; then
  if pgrep -x "Brave Browser" >/dev/null 2>&1; then
    BROWSER="Brave Browser"
  elif pgrep -x "Google Chrome" >/dev/null 2>&1; then
    BROWSER="Google Chrome"
  else
    echo "ChatGPT auto-done skipped: neither Brave Browser nor Google Chrome is running."
    exit 0
  fi
fi

JS_CODE="(function(){const el=document.querySelector('#prompt-textarea')||document.querySelector('[contenteditable=\"true\"]')||document.querySelector('textarea');if(!el)return 'NO_COMPOSER';el.focus();if(el.tagName==='TEXTAREA'){const d=Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value');if(d&&d.set){d.set.call(el,'done');}else{el.value='done';}el.dispatchEvent(new Event('input',{bubbles:true}));}else{document.execCommand('selectAll',false,null);document.execCommand('insertText',false,'done');el.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:'done'}));}setTimeout(function(){const btn=document.querySelector('button[data-testid=\"send-button\"]')||document.querySelector('button[aria-label*=\"Send\"]')||document.querySelector('button[aria-label*=\"send\"]');if(btn&&!btn.disabled){btn.click();}},700);return 'OK';})()"

set +e
OUTPUT="$(osascript - "$JS_CODE" <<APPLESCRIPT 2>&1
on run argv
	set jsCode to item 1 of argv
	try
		tell application "$BROWSER"
			if not running then return "SKIP: browser is not running"
			if (count of windows) = 0 then return "SKIP: browser has no windows"

			set activeURL to URL of active tab of front window
			if activeURL does not contain "chatgpt.com" then
				return "SKIP: front browser tab is not ChatGPT"
			end if

			tell active tab of front window
				set resultText to execute javascript jsCode
			end tell
			return resultText
		end tell
	on error errMsg number errNum
		return "ERROR " & errNum & ": " & errMsg
	end try
end run
APPLESCRIPT
)"
STATUS=$?
set -e

if [[ "$STATUS" -ne 0 || "$OUTPUT" == ERROR* ]]; then
  echo "ChatGPT auto-done skipped in $BROWSER: $OUTPUT"
  echo "$BROWSER setup: View > Developer > Allow JavaScript from Apple Events."
  echo "macOS may also ask Terminal to control $BROWSER; allow it in Privacy & Security > Automation."
  exit 0
fi

case "$OUTPUT" in
  OK)
    echo "ChatGPT fast-mode notification sent through $BROWSER: done"
    ;;
  NO_COMPOSER)
    echo "ChatGPT auto-done skipped: composer not found (ChatGPT UI may have changed)."
    ;;
  SKIP:*)
    echo "ChatGPT auto-done skipped: ${OUTPUT#SKIP: }"
    ;;
  *)
    echo "ChatGPT auto-done result from $BROWSER: $OUTPUT"
    ;;
esac

exit 0
