#!/bin/bash
# Kill any existing timereg daemon and start a new one detached.
# Logs go to ~/.config/timereg/daemon.log

pkill -f "timereg daemon" 2>/dev/null
sleep 0.2

LOG_DIR="${HOME}/.config/timereg"
mkdir -p "$LOG_DIR"

nohup timereg daemon > "$LOG_DIR/daemon.log" 2>&1 &
echo "Daemon started (pid: $!, log: $LOG_DIR/daemon.log)"
