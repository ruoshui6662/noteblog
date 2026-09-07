#!/bin/sh
set -eu

# A bind mount replaces the image's /data layer. Prepare the mounted tree as
# root, then prefer the unprivileged process when the host ACL permits it.
mkdir -p /data/content /data/media /data/backups

if su-exec 10001:10001 sh -c 'test -w /data && test -w /data/content && test -w /data/media && test -w /data/backups'; then
  exec su-exec 10001:10001 /usr/local/bin/markdown-docs "$@"
fi

echo "warning: /data is not writable by UID 10001; running the service as root because the host mount controls permissions" >&2
exec /usr/local/bin/markdown-docs "$@"
