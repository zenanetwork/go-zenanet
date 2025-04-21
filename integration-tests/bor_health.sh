#!/bin/bash
set -e

while true
do
    peers=$(docker exec zena0 bash -c "zena attach /var/lib/zena/data/zena.ipc -exec 'admin.peers'")
    block=$(docker exec zena0 bash -c "zena attach /var/lib/zena/data/zena.ipc -exec 'eth.blockNumber'")

    if [[ -n "$peers" ]] && [[ -n "$block" ]]; then
        break
    fi
done

echo "$peers"
echo "$block"
