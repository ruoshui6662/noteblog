#!/bin/sh
set -eu

# A bind mount replaces the image's /data layer. Prepare the mounted tree as
# root, then keep the HTTP process unprivileged.
mkdir -p /data/content /data/media /data/backups
chown 10001:10001 /data /data/content /data/media /data/backups

if [ -e /data/noteblog.db ]; then
  chown 10001:10001 /data/noteblog.db
fi

exec su-exec 10001:10001 /usr/local/bin/markdown-docs "$@"
