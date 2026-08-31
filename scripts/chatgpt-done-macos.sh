#!/usr/bin/env bash
set -u

# Best-effort fast-mode helper for macOS + Google Chrome.
# It intentionally does nothing unless the FRONT Chrome tab is chatgpt.com.
# Failure to notify ChatGPT must never fail a BTC research run.

if [[ "$(uname -s)" != "Darwin" ]]; then
  exit 0
fi

if ! command -v osascript >/dev/null 2>&1; then
  exit 0
fi

set +e
OUTPUT="$(osascript <<'APPLESCRIPT' 2>&1
try
	tell application "Google Chrome"
		if not running then return "SKIP: Google Chrome is not running"
		if (count of windows) = 0 then return "SKIP: Google Chrome has no windows"

		set activeTab to active tab of front window
		set activeURL to URL of activeTab
		if activeURL does not contain "chatgpt.com" then
			return "SKIP: front Chrome tab is not ChatGPT"
		end if

		set jsCode to "(function(){const el=document.querySelector('#prompt-textarea')||document.querySelector('[contenteditable=\"true\"]')||document.querySelector('textarea');if(!el)return 'NO_COMPOSER';el.focus();if(el.tagName==='TEXTAREA'){const setter=Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set;setter.call(el,'done');el.dispatchEvent(new Event('input',{bubbles:true}));}else{document.execCommand('selectAll',false,null);document.execCommand('insertText',false,'done');el.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:'done'}));}setTimeout(function(){const btn=document.querySelector('button[data-testid=\"send-button\"]')||document.querySelector('button[aria-label*=\"Send\"]')||document.querySelector('button[aria-label*=\"send\"]');if(btn&&!btn.disabled){btn.click();}},500);return 'OK';})()"
		set resultText to execute activeTab javascript jsCode
		return resultText
	end tell
on error errMsg number errNum
	return "ERROR " & errNum & ": " & errMsg
end try
APPLESCRIPT
)"
STATUS=$?
set -e

if [[ "$STATUS" -ne 0 || "$OUTPUT" == ERROR* ]]; then
  echo "ChatGPT auto-done skipped: $OUTPUT"
  echo "Chrome setup: View > Developer > Allow JavaScript from Apple Events."
  echo "macOS may also ask Terminal to control Google Chrome; allow it in Privacy & Security > Automation."
  exit 0
fi

case "$OUTPUT" in
  OK)
    echo "ChatGPT fast-mode notification sent: done"
    ;;
  NO_COMPOSER)
    echo "ChatGPT auto-done skipped: composer not found (ChatGPT UI may have changed)."
    ;;
  SKIP:*)
    echo "ChatGPT auto-done skipped: ${OUTPUT#SKIP: }"
    ;;
  *)
    echo "ChatGPT auto-done result: $OUTPUT"
    ;;
esac

exit 0
