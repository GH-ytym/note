#!/usr/bin/env bash
# Run as root after platform provisioning. Staging must contain images.tar,
# compose.yaml and SHA256SUMS; app source and release metadata are retained too.
set -euo pipefail
mode=${1:---check}
stage=$(realpath "${2:?staging directory required}")
release=${3:?release name required}
[[ "$release" =~ ^[0-9]{8}-[0-9]{4}$ ]] || exit 2
case "$mode" in --check|--apply) ;; *) exit 2 ;; esac
test "$(id -u)" = 0
mountpoint -q /data
test -f /data/projects/note/.env
(cd "$stage" && sha256sum -c SHA256SUMS)
uid=$(id -u note-svc)
as() { sudo -u note-svc env XDG_RUNTIME_DIR="/run/user/$uid" DOCKER_HOST="unix:///run/user/$uid/docker.sock" "$@"; }
as docker info --format '{{.ServerVersion}}'
NOTE_IMAGE="note-api:$release" docker compose --env-file /data/projects/note/.env -f "$stage/compose.yaml" config --quiet
echo "Validated Note release $release; only the note rootless project will be changed."
[ "$mode" = --apply ] || exit 0
dest="/data/projects/note/releases/$release"
test ! -e "$dest"
install -d -m 0700 -o note-svc -g note-svc "$dest"
for file in compose.yaml SHA256SUMS source.tar.gz release.json; do
  install -m 0600 -o note-svc -g note-svc "$stage/$file" "$dest/$file"
done
printf 'NOTE_IMAGE=note-api:%s\n' "$release" > "$dest/release.env"
chown note-svc:note-svc "$dest/release.env"
chmod 0600 "$dest/release.env"
# stdin avoids granting the service identity access to the uploader's private staging.
as docker load < "$stage/images.tar"
old=$(readlink -f /data/projects/note/current || true)
compose() { as docker compose --env-file /data/projects/note/.env --env-file "$1/release.env" -f "$1/compose.yaml" "${@:2}"; }
if ! compose "$dest" up -d --wait --wait-timeout 90; then
  echo 'New release failed its health checks.' >&2
  if [ -n "$old" ]; then compose "$old" up -d --wait --wait-timeout 90; fi
  exit 1
fi
ln -sfn "$dest" /data/projects/note/current
compose "$dest" ps
curl --fail --silent http://172.18.0.1:18094/api/ping
echo " Note release $release is running."
