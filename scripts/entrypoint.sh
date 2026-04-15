#!/bin/sh
# Usage: container entrypoint. Runs as root, fixes ownership of bind-mounted
# paths to PUID:PGID (default 1000:1000), then drops privileges to that uid
# and execs the imghost server.
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Only chown the mount roots, not their contents. Nested bind mounts may
# point at directories owned by other users (shared NAS, another service's
# data) and must not be silently rewritten.
chown "$PUID:$PGID" /data /var/lib/imghost

exec su-exec "$PUID:$PGID" /usr/local/bin/imghost
