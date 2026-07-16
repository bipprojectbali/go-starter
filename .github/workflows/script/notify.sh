#!/bin/bash

: "${TELEGRAM_TOKEN:?TELEGRAM_TOKEN tidak di-set}"
: "${TELEGRAM_CHAT_ID:?TELEGRAM_CHAT_ID tidak di-set}"
: "${NOTIFY_STATUS:?NOTIFY_STATUS tidak di-set}"
: "${NOTIFY_WORKFLOW:?NOTIFY_WORKFLOW tidak di-set}"

if [ "$NOTIFY_STATUS" = "success" ]; then
  ICON="✅"
  TEXT="${ICON} *${NOTIFY_WORKFLOW}* berhasil!"
else
  ICON="❌"
  TEXT="${ICON} *${NOTIFY_WORKFLOW}* gagal!"
fi

if [ -n "$NOTIFY_DETAIL" ]; then
  TEXT="${TEXT}
${NOTIFY_DETAIL}"
fi

curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_TOKEN}/sendMessage" \
  -H "Content-Type: application/json" \
  -d "$(jq -n \
    --arg chat_id "$TELEGRAM_CHAT_ID" \
    --arg text "$TEXT" \
    '{chat_id: $chat_id, text: $text, parse_mode: "Markdown"}')"
