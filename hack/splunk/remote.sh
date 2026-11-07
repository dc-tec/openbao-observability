#!/usr/bin/env bash
# Copy public source plus ignored credentials to an authorized SSH test host.
set -euo pipefail
if [ "$#" -ne 2 ] || [[ ! "$1" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]]; then
  echo 'Usage: bash hack/splunk/remote.sh SSH_ALIAS up|status|validate|stop' >&2
  exit 1
fi
trial_host=$1
trial_action=$2
trial_repo=$(cd "$(dirname "$0")/../.." && pwd)
cd "$trial_repo"
remote='openbao-observability-splunk'
case "$trial_action" in
  up)
    test -f examples/splunk/.env || { echo 'Run local.py setup --accept-splunk-terms first.' >&2; exit 1; }
    make generate-splunk
    mkdir -p tmp
    GOOS=linux GOARCH=amd64 go build -o tmp/openbao-observability-linux-amd64 ./cmd/openbao-observability
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o tmp/splunk-archive-health ./examples/audit-archive-health/exporter
    rsync -az --exclude .git --exclude bin --exclude tmp --exclude public --exclude resources \
      --exclude fixtures/captured --exclude workstreams --exclude __pycache__ --exclude .DS_Store \
      --exclude .env --exclude '.env.*' --exclude .runtime --exclude '.venv*' --exclude dist \
      -e 'ssh -o BatchMode=yes -o ForwardAgent=no' ./ "$trial_host:$remote/"
    scp -q -o BatchMode=yes -o ForwardAgent=no tmp/openbao-observability-linux-amd64 "$trial_host:$remote/openbao-observability"
    ssh -o BatchMode=yes -o ForwardAgent=no "$trial_host" "mkdir -p $remote/examples/splunk/.runtime/bin"
    scp -q -o BatchMode=yes -o ForwardAgent=no tmp/splunk-archive-health "$trial_host:$remote/examples/splunk/.runtime/bin/archive-health"
    rsync -a --ignore-existing --chmod=F600 -e 'ssh -o BatchMode=yes -o ForwardAgent=no' \
      examples/splunk/.env "$trial_host:$remote/examples/splunk/.env"
    ssh -o BatchMode=yes -o ForwardAgent=no "$trial_host" "cd $remote; bash hack/splunk/remote-up.sh"
    ;;
  status)
    ssh -o BatchMode=yes -o ForwardAgent=no "$trial_host" "cd $remote; nix shell nixpkgs#python3 --command python3 hack/splunk/local.py status"
    ;;
  validate)
    ssh -o BatchMode=yes -o ForwardAgent=no "$trial_host" "cd $remote; ./openbao-observability validate splunk-profile && nix shell nixpkgs#python3 --command bash -ec 'python3 hack/splunk/local.py indexes; python3 hack/splunk/local.py install; python3 hack/splunk/local.py audit; python3 hack/splunk/local.py synthetic-metrics; python3 hack/splunk/local.py synthetic-topology; python3 hack/splunk/local.py synthetic-families; python3 hack/splunk/local.py history; python3 hack/splunk/local.py verify-external; python3 hack/splunk/local.py panels; python3 hack/splunk/local.py parity'"
    ;;
  stop)
    ssh -o BatchMode=yes -o ForwardAgent=no "$trial_host" "cd $remote; nix shell nixpkgs#python3 nixpkgs#podman --command python3 hack/splunk/local.py external-stop --engine podman"
    ssh -o BatchMode=yes -o ForwardAgent=no "$trial_host" "cd $remote; nix shell nixpkgs#podman nixpkgs#podman-compose --command bash -ec 'podman stop --time 60 openbao-observability_splunk-otel_1 openbao-splunk-trial; podman-compose --env-file examples/splunk/.env -f examples/docker-compose/compose.yaml -f examples/splunk/compose.yaml -p openbao-observability stop prometheus openbao-node2 openbao-node1 openbao-node0 postgres'"
    ;;
  *) echo 'Unknown action.' >&2; exit 1 ;;
esac
