OPENBAO_VERSION ?= 2.5.4
OPENBAO_IMAGE ?= quay.io/openbao/openbao:$(OPENBAO_VERSION)
PROMETHEUS_IMAGE ?= prom/prometheus:v3.11.2
OPENBAO_PORT_BASE ?= 18220
OPENBAO_ROOT_TOKEN ?= root
FIXTURE_DIR ?= fixtures/captured/openbao-$(OPENBAO_VERSION)
COMPOSE_FILE ?= examples/docker-compose/compose.yaml
COMPOSE_PROJECT_DIR ?= examples/docker-compose
GO ?= go
PROMTOOL ?= docker run --rm --entrypoint promtool -v "$(CURDIR):/workspace:ro" "$(PROMETHEUS_IMAGE)"
PROMETHEUS_URL ?= http://127.0.0.1:19090
LOKI_URL ?= http://127.0.0.1:13100
DASHBOARD_CONTRACTS ?= contracts/dashboards/openbao-overview.yaml,contracts/dashboards/openbao-ha-raft.yaml,contracts/dashboards/openbao-audit-overview.yaml,contracts/dashboards/openbao-operational-logs.yaml,contracts/dashboards/openbao-audit-investigation.yaml,contracts/dashboards/openbao-auth-identity.yaml,contracts/dashboards/openbao-token-lease-lifecycle.yaml
GENERATED_DASHBOARDS ?= generated/grafana/openbao-overview.json,generated/grafana/openbao-ha-raft.json,generated/grafana/openbao-audit-overview.json,generated/grafana/openbao-operational-logs.json,generated/grafana/openbao-audit-investigation.json,generated/grafana/openbao-auth-identity.json,generated/grafana/openbao-token-lease-lifecycle.json

.PHONY: compose-config compose-down compose-reset compose-up contracts-verify fixtures-openbao generate test test-fixtures test-unit validate-dashboard-queries validate-generated verify verify-live

compose-config:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" config

compose-up:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" up -d

compose-down:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" down

compose-reset:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" down --volumes

contracts-verify:
	$(GO) run ./cmd/openbao-observability contracts verify \
		--contract "contracts/metrics/openbao-core.yaml" \
		--fixtures "$(FIXTURE_DIR)"
	$(GO) run ./cmd/openbao-observability contracts verify-alerts \
		--contract "contracts/alerts/critical.yaml" \
		--severity "critical"
	$(GO) run ./cmd/openbao-observability contracts verify-alerts \
		--contract "contracts/alerts/warning.yaml" \
		--severity "warning"
	$(GO) run ./cmd/openbao-observability contracts verify-streams \
		--contract "contracts/streams/log-streams.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-streams \
		--contract "contracts/streams/log-streams.yaml" \
		--alert-contract "contracts/alerts/warning.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-overview.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-ha-raft.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-audit-overview.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-operational-logs.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-audit-investigation.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-auth-identity.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-token-lease-lifecycle.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-repository

fixtures-openbao:
	$(GO) run ./cmd/openbao-observability fixtures capture \
		--version "$(OPENBAO_VERSION)" \
		--image "$(OPENBAO_IMAGE)" \
		--output "$(FIXTURE_DIR)" \
		--port-base "$(OPENBAO_PORT_BASE)" \
		--root-token "$(OPENBAO_ROOT_TOKEN)"

generate:
	$(GO) run ./cmd/openbao-observability generate prometheus-rules \
		--contract "contracts/metrics/openbao-core.yaml" \
		--output "generated/prometheusrules/openbao-recording-rules.yaml" \
		--rule-output "generated/prometheus/openbao-recording-rules.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/critical.yaml" \
		--prometheus-name "openbao-alerts" \
		--loki-name "openbao-loki-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-alerts.yaml" \
		--loki-output "generated/loki/openbao-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/warning.yaml" \
		--prometheus-name "openbao-warning-alerts" \
		--loki-name "openbao-loki-warning-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-warning-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-warning-alerts.yaml" \
		--loki-output "generated/loki/openbao-warning-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-overview.yaml" \
		--output "generated/grafana/openbao-overview.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-ha-raft.yaml" \
		--output "generated/grafana/openbao-ha-raft.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-audit-overview.yaml" \
		--output "generated/grafana/openbao-audit-overview.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-operational-logs.yaml" \
		--output "generated/grafana/openbao-operational-logs.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-audit-investigation.yaml" \
		--output "generated/grafana/openbao-audit-investigation.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-auth-identity.yaml" \
		--output "generated/grafana/openbao-auth-identity.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-token-lease-lifecycle.yaml" \
		--output "generated/grafana/openbao-token-lease-lifecycle.json"

test: test-fixtures contracts-verify validate-generated test-unit

verify: generate test
	git diff --exit-code -- generated

verify-live: validate-dashboard-queries

test-unit:
	$(GO) test ./...

test-fixtures:
	$(GO) run ./cmd/openbao-observability fixtures verify \
		--version "$(OPENBAO_VERSION)" \
		--dir "$(FIXTURE_DIR)"

validate-generated:
	$(PROMTOOL) check rules \
		/workspace/generated/prometheus/openbao-recording-rules.yaml \
		/workspace/generated/prometheus/openbao-alerts.yaml \
		/workspace/generated/prometheus/openbao-warning-alerts.yaml

validate-dashboard-queries:
	$(GO) run ./cmd/openbao-observability validate dashboard-queries \
		--contracts "$(DASHBOARD_CONTRACTS)" \
		--generated "$(GENERATED_DASHBOARDS)" \
		--prometheus-url "$(PROMETHEUS_URL)" \
		--loki-url "$(LOKI_URL)"
