#!/bin/sh
# Drop to the numeric owner of the repository before reading owner-only keys.
set -eu
if [ ! -f /project-owner ]; then
    echo "mrkvpn: missing read-only /project-owner bind mount" >&2
    exit 1
fi
OWNER_UID="$(stat -c '%u' /project-owner)"
OWNER_GID="$(stat -c '%g' /project-owner)"
exec su-exec "$OWNER_UID:$OWNER_GID" /usr/local/bin/mrkvpn "$@"
