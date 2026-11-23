#!/usr/bin/env bash
# A consistent SQLite backup: briefly stop API; always restart on exit.
set -euo pipefail
cd "$(dirname "$(realpath "$0")")"
mode=${1:---check}
case "$mode" in --check|--apply) ;; *) echo 'Usage: bash backup.sh --check|--apply' >&2; exit 2;; esac
docker compose config --quiet
id=$(docker compose ps -q api)
test -n "$id" || { echo 'API must be running before backup' >&2; exit 1; }
echo 'Stop Note API, copy /data (SQLite), restart, then archive. COS objects and Redis sessions are not included.'
[ "$mode" = --apply ] || exit 0
umask 077
mkdir -p backups
dest=$(mktemp -d "$PWD/backups/note-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXXXX")
trap 'docker compose start api >/dev/null' EXIT
docker compose stop -t 30 api
docker cp "$id:/data/." "$dest/data"
cp .env "$dest/env"
docker compose start api >/dev/null
trap - EXIT
tar -C "$dest" -czf "$dest.tar.gz" data env
sha256sum "$dest.tar.gz" > "$dest.tar.gz.sha256"
echo "Backup: $dest.tar.gz (includes secrets; store privately off this server)."
# Keep unpacked copy too; never delete or rotate user backups automatically.
