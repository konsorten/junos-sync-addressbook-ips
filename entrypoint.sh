#!/bin/bash

set -e

BIN=/go/bin/junos-sync-addressbook-ips

if [ ! -x "$BIN" ]; then
    echo "Missing executable: $BIN"
    exit 1
fi

FAILED=0

for cfg in /etc/juniper-address-set-mapping/*; do
    echo "Processing $cfg ..."

    set +e
    JUNIPER_ADDRESS_SET="$(basename $cfg)" IPS_SOURCE_URL="$(cat $cfg)" $BIN

    if [ $? -ne 0 ]; then
        let FAILED++
    fi
    set -e
done

echo "Failed processings: $FAILED"
echo "DONE."

exit $FAILED
