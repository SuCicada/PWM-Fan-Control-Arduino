#!/usr/bin/env bash
set -euo pipefail

PORT=/dev/ttyUSB0
MAX_RETRIES=5
RETRY_SLEEP=2

hex_file=${1:-}
if [ -z "$hex_file" ]; then
    echo "Hex file is required"
    exit 1
fi

free_port() {
    local used_pid
    used_pid=$(lsof -t "$PORT" 2>/dev/null || true)
    if [ -n "$used_pid" ]; then
        echo "Killing process(es) on $PORT: $used_pid"
        # shellcheck disable=SC2086
        kill -9 $used_pid 2>/dev/null || true
        sleep 1
    else
        echo "No process found on $PORT"
    fi
}

attempt=1
while [ "$attempt" -le "$MAX_RETRIES" ]; do
    echo "Upload attempt $attempt/$MAX_RETRIES ..."
    free_port

    if avrdude -p m328p -c arduino -P "$PORT" -b 115200 -Uflash:w:"${hex_file}":i; then
        echo "Done"
        exit 0
    fi

    echo "avrdude failed (busy/sync?). retrying in ${RETRY_SLEEP}s ..."
    attempt=$((attempt + 1))
    sleep "$RETRY_SLEEP"
done

echo "Upload failed after $MAX_RETRIES attempts"
exit 1
