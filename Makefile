OPENBAO_VERSION ?= 2.5.4
OPENBAO_IMAGE ?= quay.io/openbao/openbao:$(OPENBAO_VERSION)
POSTGRES_IMAGE ?= postgres:17-alpine
PROMETHEUS_IMAGE ?= prom/prometheus:v3.11.2
OPENBAO_PORT_BASE ?= 18220
OPENBAO_ROOT_TOKEN ?= root
FIXTURE_DIR ?= fixtures/captured/openbao-$(OPENBAO_VERSION)
COMPOSE_FILE ?= examples/docker-compose/compose.yaml
COMPOSE_AUDIT_ARCHIVE_FILE ?= examples/docker-compose/compose.audit-archive.yaml
COMPOSE_PROJECT_DIR ?= examples/docker-compose
GO ?= go
PROMTOOL ?= docker run --rm --entrypoint promtool -v "$(CURDIR):/workspace:ro" "$(PROMETHEUS_IMAGE)"
PROMETHEUS_URL ?= http://127.0.0.1:19090
LOKI_URL ?= http://127.0.0.1:13100
DASHBOARD_CONTRACTS ?= contracts/dashboards/openbao-overview.yaml,contracts/dashboards/openbao-ha-raft.yaml,contracts/dashboards/openbao-audit-overview.yaml,contracts/dashboards/openbao-operational-logs.yaml,contracts/dashboards/openbao-audit-investigation.yaml,contracts/dashboards/openbao-auth-identity.yaml,contracts/dashboards/openbao-token-lease-lifecycle.yaml,contracts/dashboards/openbao-database-secrets.yaml,contracts/dashboards/openbao-transit.yaml,contracts/dashboards/openbao-pki.yaml,contracts/dashboards/openbao-secret-engines-mounts.yaml,contracts/dashboards/openbao-runtime-storage.yaml,contracts/dashboards/openbao-kubernetes-platform.yaml,contracts/dashboards/openbao-slo-availability.yaml
GENERATED_DASHBOARDS ?= generated/grafana/openbao-overview.json,generated/grafana/openbao-ha-raft.json,generated/grafana/openbao-audit-overview.json,generated/grafana/openbao-operational-logs.json,generated/grafana/openbao-audit-investigation.json,generated/grafana/openbao-auth-identity.json,generated/grafana/openbao-token-lease-lifecycle.json,generated/grafana/openbao-database-secrets.json,generated/grafana/openbao-transit.json,generated/grafana/openbao-pki.json,generated/grafana/openbao-secret-engines-mounts.json,generated/grafana/openbao-runtime-storage.json,generated/grafana/openbao-kubernetes-platform.json,generated/grafana/openbao-slo-availability.json

.PHONY: compose-audit-archive-config compose-audit-archive-down compose-audit-archive-reset compose-audit-archive-up compose-config compose-down compose-reset compose-up contracts-verify docs-verify fixtures-openbao fixtures-scenarios generate test test-fixtures test-unit validate-dashboard-queries validate-generated verify verify-live

compose-config:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" config

compose-audit-archive-config:
	PROMETHEUS_CONFIG=./prometheus/prometheus.audit-archive.yml docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" -f "$(COMPOSE_AUDIT_ARCHIVE_FILE)" --profile audit-archive config

compose-up:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" up -d

compose-audit-archive-up:
	PROMETHEUS_CONFIG=./prometheus/prometheus.audit-archive.yml docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" -f "$(COMPOSE_AUDIT_ARCHIVE_FILE)" --profile audit-archive up -d --build

compose-down:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" down

compose-audit-archive-down:
	PROMETHEUS_CONFIG=./prometheus/prometheus.audit-archive.yml docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" -f "$(COMPOSE_AUDIT_ARCHIVE_FILE)" --profile audit-archive down

compose-reset:
	docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" down --volumes

compose-audit-archive-reset:
	PROMETHEUS_CONFIG=./prometheus/prometheus.audit-archive.yml docker compose --project-directory "$(COMPOSE_PROJECT_DIR)" -f "$(COMPOSE_FILE)" -f "$(COMPOSE_AUDIT_ARCHIVE_FILE)" --profile audit-archive down --volumes

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
	$(GO) run ./cmd/openbao-observability contracts verify-alerts \
		--contract "contracts/alerts/security.yaml" \
		--severity "warning"
	$(GO) run ./cmd/openbao-observability contracts verify-streams \
		--contract "contracts/streams/log-streams.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-streams \
		--contract "contracts/streams/log-streams.yaml" \
		--alert-contract "contracts/alerts/warning.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-streams \
		--contract "contracts/streams/log-streams.yaml" \
		--alert-contract "contracts/alerts/security.yaml"
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
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-database-secrets.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-transit.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-pki.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-secret-engines-mounts.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-runtime-storage.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-kubernetes-platform.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-dashboards \
		--contract "contracts/dashboards/openbao-slo-availability.yaml"
	$(GO) run ./cmd/openbao-observability contracts verify-repository

fixtures-openbao:
	$(GO) run ./cmd/openbao-observability fixtures capture \
		--version "$(OPENBAO_VERSION)" \
		--image "$(OPENBAO_IMAGE)" \
		--postgres-image "$(POSTGRES_IMAGE)" \
		--output "$(FIXTURE_DIR)" \
		--port-base "$(OPENBAO_PORT_BASE)" \
		--root-token "$(OPENBAO_ROOT_TOKEN)"

fixtures-scenarios:
	$(GO) run ./cmd/openbao-observability fixtures scenario \
		--output "$(FIXTURE_DIR)/metadata/openbao-$(OPENBAO_VERSION)-compose-scenario.json"

generate:
	$(GO) run ./cmd/openbao-observability generate prometheus-rules \
		--contract "contracts/metrics/openbao-core.yaml" \
		--output "generated/prometheusrules/openbao-recording-rules.yaml" \
		--rule-output "generated/prometheus/openbao-recording-rules.yaml"
	$(GO) run ./cmd/openbao-observability generate prometheus-rules \
		--contract "contracts/metrics/openbao-core.yaml" \
		--source-prefix "vault" \
		--output "generated/prometheusrules/vault-prefix/openbao-recording-rules.yaml" \
		--rule-output "generated/prometheus/vault-prefix/openbao-recording-rules.yaml"
	$(GO) run ./cmd/openbao-observability generate prometheus-rules \
		--contract "contracts/metrics/openbao-core.yaml" \
		--source-prefix "openbao" \
		--output "generated/prometheusrules/openbao-prefix/openbao-recording-rules.yaml" \
		--rule-output "generated/prometheus/openbao-prefix/openbao-recording-rules.yaml"
	$(GO) run ./cmd/openbao-observability generate compatibility-matrix \
		--contract "contracts/metrics/openbao-core.yaml" \
		--fixtures "$(FIXTURE_DIR)" \
		--output "generated/docs/metric-compatibility-matrix.md"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/critical.yaml" \
		--prometheus-name "openbao-alerts" \
		--loki-name "openbao-loki-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-alerts.yaml" \
		--loki-output "generated/loki/openbao-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/critical.yaml" \
		--source-prefix "vault" \
		--prometheus-name "openbao-alerts" \
		--loki-name "openbao-loki-alerts" \
		--prometheus-output "generated/prometheusrules/vault-prefix/openbao-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/vault-prefix/openbao-alerts.yaml" \
		--loki-output "generated/loki/vault-prefix/openbao-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/critical.yaml" \
		--source-prefix "openbao" \
		--prometheus-name "openbao-alerts" \
		--loki-name "openbao-loki-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-prefix/openbao-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-prefix/openbao-alerts.yaml" \
		--loki-output "generated/loki/openbao-prefix/openbao-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/warning.yaml" \
		--prometheus-name "openbao-warning-alerts" \
		--loki-name "openbao-loki-warning-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-warning-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-warning-alerts.yaml" \
		--loki-output "generated/loki/openbao-warning-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/warning.yaml" \
		--source-prefix "vault" \
		--prometheus-name "openbao-warning-alerts" \
		--loki-name "openbao-loki-warning-alerts" \
		--prometheus-output "generated/prometheusrules/vault-prefix/openbao-warning-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/vault-prefix/openbao-warning-alerts.yaml" \
		--loki-output "generated/loki/vault-prefix/openbao-warning-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/warning.yaml" \
		--source-prefix "openbao" \
		--prometheus-name "openbao-warning-alerts" \
		--loki-name "openbao-loki-warning-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-prefix/openbao-warning-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-prefix/openbao-warning-alerts.yaml" \
		--loki-output "generated/loki/openbao-prefix/openbao-warning-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/security.yaml" \
		--prometheus-name "openbao-security-alerts" \
		--loki-name "openbao-loki-security-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-security-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-security-alerts.yaml" \
		--loki-output "generated/loki/openbao-security-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/security.yaml" \
		--source-prefix "vault" \
		--prometheus-name "openbao-security-alerts" \
		--loki-name "openbao-loki-security-alerts" \
		--prometheus-output "generated/prometheusrules/vault-prefix/openbao-security-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/vault-prefix/openbao-security-alerts.yaml" \
		--loki-output "generated/loki/vault-prefix/openbao-security-alerts.yaml"
	$(GO) run ./cmd/openbao-observability generate alert-rules \
		--contract "contracts/alerts/security.yaml" \
		--source-prefix "openbao" \
		--prometheus-name "openbao-security-alerts" \
		--loki-name "openbao-loki-security-alerts" \
		--prometheus-output "generated/prometheusrules/openbao-prefix/openbao-security-alerts.yaml" \
		--prometheus-rule-output "generated/prometheus/openbao-prefix/openbao-security-alerts.yaml" \
		--loki-output "generated/loki/openbao-prefix/openbao-security-alerts.yaml"
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
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-database-secrets.yaml" \
		--output "generated/grafana/openbao-database-secrets.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-transit.yaml" \
		--output "generated/grafana/openbao-transit.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-pki.yaml" \
		--output "generated/grafana/openbao-pki.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-secret-engines-mounts.yaml" \
		--output "generated/grafana/openbao-secret-engines-mounts.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-runtime-storage.yaml" \
		--output "generated/grafana/openbao-runtime-storage.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-kubernetes-platform.yaml" \
		--output "generated/grafana/openbao-kubernetes-platform.json"
	$(GO) run ./cmd/openbao-observability generate grafana-dashboard \
		--contract "contracts/dashboards/openbao-slo-availability.yaml" \
		--output "generated/grafana/openbao-slo-availability.json"

test: test-fixtures contracts-verify docs-verify validate-generated test-unit

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
		/workspace/generated/prometheus/openbao-warning-alerts.yaml \
		/workspace/generated/prometheus/openbao-security-alerts.yaml \
		/workspace/generated/prometheus/vault-prefix/openbao-recording-rules.yaml \
		/workspace/generated/prometheus/vault-prefix/openbao-alerts.yaml \
		/workspace/generated/prometheus/vault-prefix/openbao-warning-alerts.yaml \
		/workspace/generated/prometheus/vault-prefix/openbao-security-alerts.yaml \
		/workspace/generated/prometheus/openbao-prefix/openbao-recording-rules.yaml \
		/workspace/generated/prometheus/openbao-prefix/openbao-alerts.yaml \
		/workspace/generated/prometheus/openbao-prefix/openbao-warning-alerts.yaml \
		/workspace/generated/prometheus/openbao-prefix/openbao-security-alerts.yaml

docs-verify:
	$(GO) run ./cmd/openbao-observability validate docs \
		--repository-root "." \
		--docs-root "docs"

validate-dashboard-queries:
	$(GO) run ./cmd/openbao-observability validate dashboard-queries \
		--contracts "$(DASHBOARD_CONTRACTS)" \
		--generated "$(GENERATED_DASHBOARDS)" \
		--prometheus-url "$(PROMETHEUS_URL)" \
		--loki-url "$(LOKI_URL)"
