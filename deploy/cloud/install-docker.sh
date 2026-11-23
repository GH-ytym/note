#!/usr/bin/env bash
# Ubuntu packages, delivered through the host's configured signed apt mirror.
set -euo pipefail
. /etc/os-release
test "$ID" = ubuntu
sudo -n true
sudo apt-get update -o Acquire::Retries=2 -o Acquire::https::Timeout=30 -o APT::Update::Error-Mode=any
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends docker.io docker-compose-v2
sudo systemctl enable --now docker
sudo docker version
sudo docker compose version
sudo systemctl is-active docker
