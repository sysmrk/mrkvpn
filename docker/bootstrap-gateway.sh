#!/bin/sh
# Local bootstrap only. Never creates a client private key or touches routes.
set -eu
umask 077

mkdir -p /config /secrets /output
chmod 0700 /config /secrets /output

for p in /secrets/gateway.key /secrets/gateway.pub /config/gateway.json; do
    if [ -L "$p" ]; then
        echo "mrkvpn init: refusing symlink: $p" >&2
        exit 1
    fi
done

if [ ! -e /secrets/gateway.key ]; then
    wg genkey > /secrets/gateway.key
    chmod 0600 /secrets/gateway.key
    echo "Created gateway private key (not displayed)."
else
    echo "Existing gateway private key preserved."
fi

if ! wg pubkey < /secrets/gateway.key > /dev/null; then
    echo "mrkvpn init: gateway private key is invalid" >&2
    exit 1
fi

if [ ! -e /secrets/gateway.pub ]; then
    wg pubkey < /secrets/gateway.key > /secrets/gateway.pub
    chmod 0600 /secrets/gateway.pub
fi

if [ ! -e /config/gateway.json ]; then
    cat > /config/gateway.json <<'JSON'
{
  "interface_name": "wg0",
  "address": "10.77.0.1/24",
  "listen_port": 51820,
  "private_key_file": "/secrets/gateway.key",
  "peers": []
}
JSON
    chmod 0600 /config/gateway.json
    echo "Created /config/gateway.json with zero clients."
else
    echo "Existing gateway JSON preserved."
fi

echo "Bootstrap complete. Add client PUBLIC keys locally before enabling clients."
echo "The gateway public key is in .local/secrets/gateway.pub."
