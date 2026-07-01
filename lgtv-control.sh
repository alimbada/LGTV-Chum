#!/bin/bash

command -v jq >/dev/null 2>&1 || { echo >&2 "Install jq first"; exit 1; }

wait_for_network() {
    local max_attempts=15
    local attempt=1
    local gateway

    while [ $attempt -le $max_attempts ]; do
        # Dynamically find the default gateway (usually your router)
        gateway=$(ip route | grep default | awk '{print $3}')
      
        # If a gateway is found and successfully responds to a ping, we're online
        if [ -n "$gateway" ] && ping -c 1 -W 1 "$gateway" >/dev/null 2>&1; then
            echo "[$(date)] Network gateway found ($gateway). Proceeding."
            return 0
        fi
      
        # If not, wait 1 second and try again
        sleep 1
        attempt=$((attempt + 1))
    done

    echo "[$(date)] Error: Network timed out after $max_attempts seconds." >&2
    return 1
}

source "$HOME/lgtv-venv/bin/activate"

jq_cmd() {
    jq --raw-output "$@"
}

exec_lgtv() {
    response="$(lgtv --name MyTV --ssl "$@" | head -n 1 )"
    if [ "$(echo "$response" | jq_cmd .type)" == "error" ]; then
        echo "$response" | jq_cmd .error
        exit 1
    fi

    echo "$response"
}

if wait_for_network; then
    exec_lgtv $@
else
    echo "Aborting LGTV command due to missing network context."
    exit 1
fi
