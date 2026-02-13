#!/bin/bash
# PostToolUse hook: run goimports on .go files after Write/Edit
FILE=$(jq -r '.tool_input.file_path // empty')
if [ -n "$FILE" ] && echo "$FILE" | grep -q '\.go$' && [ -f "$FILE" ]; then
    goimports -w "$FILE"
fi
