#!/usr/bin/env bash
# increase-task-limit.sh — Raise systemd TasksMax for user sessions
# from the default 8137 (15% of threads-max) to 12000.
# Requires root. Safe for 8-CPU / 6.7GB machines (stays under
# the parent user-slice ceiling of 17901).

set -euo pipefail

TASKS_MAX=12000
OVERRIDE_DIR="/etc/systemd/system/user@.service.d"
OVERRIDE_FILE="${OVERRIDE_DIR}/override.conf"

echo "Current TasksMax for user@1000.service:"
systemctl show user@1000.service -p TasksMax

sudo mkdir -p "${OVERRIDE_DIR}"
sudo tee "${OVERRIDE_FILE}" > /dev/null <<EOF
[Service]
TasksMax=${TASKS_MAX}
EOF

sudo systemctl daemon-reload

echo ""
echo "New TasksMax for user@1000.service:"
systemctl show user@1000.service -p TasksMax

echo ""
echo "Applied TasksMax=${TASKS_MAX} to ${OVERRIDE_FILE}"
echo "Takes effect for new sessions. To apply to current tmux scope:"
echo "  sudo systemctl set-property user@1000.service TasksMax=${TASKS_MAX}"
