#!/bin/bash
# vzdump hook — reports backup completion to the Probaky server.
# Proxmox calls this with "job-end" after each backup job finishes.

INSTALL_DIR="/opt/probaky"
LOG_DIR="/var/log/probaky"
LOCK_FILE="$INSTALL_DIR/.probaky.lock"
LOG_FILE="$LOG_DIR/hook.log"
PENDING_FILE="$INSTALL_DIR/.report_pending"

log_msg() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"
}

send_report() {
    log_msg "INFO: Sending backup report..."
    cd "$INSTALL_DIR" || { log_msg "ERROR: Cannot access $INSTALL_DIR"; return 1; }
    ./probaky-client --vzdump-hook >> "$LOG_FILE" 2>&1
    local code=$?
    log_msg "INFO: probaky-client exited with code $code"
    return $code
}

if [ "$1" = "job-end" ]; then
    log_msg "INFO: Backup completed, reporting..."
    touch "$LOCK_FILE"
    chmod 666 "$LOCK_FILE"
    (
        flock -w 30 200 || {
            log_msg "WARN: Could not acquire lock, saving as pending"
            echo "$(date '+%Y-%m-%d %H:%M:%S')" > "$PENDING_FILE"
            exit 0
        }
        send_report
        [ -f "$PENDING_FILE" ] && rm -f "$PENDING_FILE"
    ) 200>"$LOCK_FILE"
    log_msg "INFO: Done"
fi

exit 0
