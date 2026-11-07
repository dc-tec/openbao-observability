#!/usr/bin/env bash
# Run inside the copied repository on an authorized disposable Linux test host.
set -euo pipefail
if ! command -v podman >/dev/null || ! command -v podman-compose >/dev/null || ! command -v python3 >/dev/null; then
  exec nix shell nixpkgs#podman nixpkgs#podman-compose nixpkgs#python3 nixpkgs#slirp4netns nixpkgs#fuse-overlayfs --command bash "$0"
fi
if [ "$(id -u)" = 0 ]; then
  echo 'Run this profile as an unprivileged user with rootless Podman.' >&2
  exit 1
fi
umask 077
trial_root=$(pwd)
trial_seed="$HOME/openbao-splunk-trial"
mkdir -p "$trial_seed/profile" tmp
chmod 700 "$trial_seed"
cp examples/splunk/.env "$trial_seed/.env"
cp examples/splunk/local/{start.sh,indexes.conf,policy.json} "$trial_seed/"
cp -R generated/splunk/app/openbao_observability/. "$trial_seed/profile/"
# Host persistence is a prerequisite, not a side effect of starting a trial.
if [ "$(loginctl show-user "$(id -un)" --property=Linger --value)" != yes ]; then
  echo 'Enable systemd user lingering for this test account before starting the SSH-hosted trial.' >&2
  exit 1
fi
export POSTGRES_IMAGE=docker.io/library/postgres:18-alpine
export PROMETHEUS_IMAGE=docker.io/prom/prometheus:v3.15.1
compose=(podman-compose --env-file examples/splunk/.env -f examples/docker-compose/compose.yaml
  -f examples/splunk/compose.yaml -p openbao-observability
  "--podman-pull-args=--signature-policy=$trial_seed/policy.json"
  "--podman-run-args=--signature-policy=$trial_seed/policy.json")
# Podman Compose can include credentials in its command output; keep this log private.
"${compose[@]}" up -d openbao-node0 openbao-node1 openbao-node2 prometheus >tmp/splunk-start.log 2>&1
if podman container exists openbao-splunk-trial; then
  podman start openbao-splunk-trial >/dev/null
else
  splunk_image=$(python3 hack/splunk/local.py image SPLUNK_IMAGE)
  podman run --signature-policy "$trial_seed/policy.json" --detach --name openbao-splunk-trial \
    --hostname splunk --user 0 --network openbao-observability_default --network-alias splunk \
    --env-file "$trial_seed/.env" --entrypoint /bin/sh \
    -p 127.0.0.1:18000:8000 -p 127.0.0.1:18089:8089 -p 127.0.0.1:18088:8088 \
    -v openbao-splunk-etc:/opt/splunk/etc -v openbao-splunk-var:/opt/splunk/var \
    -v "$trial_seed/profile:/openbao/profile:ro" \
    -v "$trial_seed/indexes.conf:/openbao/indexes.conf:ro" \
    -v "$trial_seed/start.sh:/openbao/start.sh:ro" \
    "$splunk_image" /openbao/start.sh >/dev/null
fi
python3 hack/splunk/local.py wait
python3 hack/splunk/local.py indexes
python3 hack/splunk/local.py install
python3 hack/splunk/local.py external --engine podman
python3 hack/splunk/local.py collector --engine podman
printf '%s\n' "Trial services started in $trial_root; API readiness can take several minutes."
