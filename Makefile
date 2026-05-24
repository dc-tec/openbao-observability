OPENBAO_VERSION ?= 2.5.4
OPENBAO_IMAGE ?= quay.io/openbao/openbao:$(OPENBAO_VERSION)
POSTGRES_IMAGE ?= postgres:17-alpine
PROMETHEUS_IMAGE ?= prom/prometheus:v3.11.2
OPENBAO_PORT_BASE ?= 18220
OPENBAO_ROOT_TOKEN ?= root
FIXTURE_DIR ?= fixtures/captured/openbao-$(OPENBAO_VERSION)
VERSION ?= 0.0.0-dev
DIST_DIR ?= dist/release
SOURCE_DATE_EPOCH ?= 0
RELEASE_BUNDLE ?= $(DIST_DIR)/openbao-observability_$(VERSION).tar.gz
COMPOSE_FILE ?= examples/docker-compose/compose.yaml
COMPOSE_AUDIT_ARCHIVE_FILE ?= examples/docker-compose/compose.audit-archive.yaml
COMPOSE_PROJECT_DIR ?= examples/docker-compose
KIND ?= kind
KUBECTL ?= kubectl
KIND_OPERATOR_PROFILE_DIR ?= examples/kubernetes/kind/operator-managed
KIND_OPERATOR_CLUSTER ?= openbao-observability
KIND_OPERATOR_NAMESPACE ?= openbao
KIND_OPERATOR_OPENBAO_CLUSTER ?= openbao-observability
KIND_OPERATOR_TENANT_NAMESPACE ?= $(KIND_OPERATOR_NAMESPACE)
KIND_OPERATOR_TENANT ?= openbao-observability
KIND_OPERATOR_PROMETHEUS_RULE_NAMESPACE ?= monitoring
KIND_OPERATOR_RULE_PROFILE ?= openbao-prefix
KIND_OPERATOR_WAIT_TIMEOUT ?= 10m
GO ?= go
HUGO_VERSION ?= v0.159.1
HUGO_RUN ?= GOFLAGS="-mod=mod" "$(GO)" run github.com/gohugoio/hugo@$(HUGO_VERSION)
DOCS_BASE_URL ?= https://dc-tec.github.io/openbao-observability/
DOCS_OUT ?= public
PROMTOOL ?= docker run --rm --entrypoint promtool -v "$(CURDIR):/workspace:ro" "$(PROMETHEUS_IMAGE)"
PROMETHEUS_URL ?= http://127.0.0.1:19090
LOKI_URL ?= http://127.0.0.1:13100
DASHBOARD_CONTRACTS ?= contracts/dashboards/openbao-overview.yaml,contracts/dashboards/openbao-ha-raft.yaml,contracts/dashboards/openbao-audit-overview.yaml,contracts/dashboards/openbao-operational-logs.yaml,contracts/dashboards/openbao-audit-investigation.yaml,contracts/dashboards/openbao-auth-identity.yaml,contracts/dashboards/openbao-token-lease-lifecycle.yaml,contracts/dashboards/openbao-database-secrets.yaml,contracts/dashboards/openbao-transit.yaml,contracts/dashboards/openbao-pki.yaml,contracts/dashboards/openbao-secret-engines-mounts.yaml,contracts/dashboards/openbao-runtime-storage.yaml,contracts/dashboards/openbao-kubernetes-platform.yaml,contracts/dashboards/openbao-slo-availability.yaml
GENERATED_DASHBOARDS ?= generated/grafana/openbao-overview.json,generated/grafana/openbao-ha-raft.json,generated/grafana/openbao-audit-overview.json,generated/grafana/openbao-operational-logs.json,generated/grafana/openbao-audit-investigation.json,generated/grafana/openbao-auth-identity.json,generated/grafana/openbao-token-lease-lifecycle.json,generated/grafana/openbao-database-secrets.json,generated/grafana/openbao-transit.json,generated/grafana/openbao-pki.json,generated/grafana/openbao-secret-engines-mounts.json,generated/grafana/openbao-runtime-storage.json,generated/grafana/openbao-kubernetes-platform.json,generated/grafana/openbao-slo-availability.json

.PHONY: checksums compose-audit-archive-config compose-audit-archive-down compose-audit-archive-reset compose-audit-archive-up compose-config compose-down compose-reset compose-up contracts-verify docs-build docs-serve docs-site-links docs-verify fixtures-openbao fixtures-scenarios generate kind-operator-api-server-endpoints kind-operator-apply kind-operator-apply-rules kind-operator-apply-tenant kind-operator-config kind-operator-down kind-operator-patch-api-server-endpoints kind-operator-up kind-operator-validate release-artifacts release-bundle test test-fixtures test-unit validate-dashboard-queries validate-generated verify verify-live

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

kind-operator-config:
	$(KUBECTL) kustomize "$(KIND_OPERATOR_PROFILE_DIR)"

kind-operator-up:
	$(KIND) create cluster \
		--name "$(KIND_OPERATOR_CLUSTER)" \
		--config "$(KIND_OPERATOR_PROFILE_DIR)/kind-config.yaml"

kind-operator-apply:
	$(KUBECTL) apply -k "$(KIND_OPERATOR_PROFILE_DIR)"

kind-operator-apply-tenant:
	$(KUBECTL) apply -f "$(KIND_OPERATOR_PROFILE_DIR)/tenant.yaml"

kind-operator-api-server-endpoints:
	@$(KUBECTL) get endpoints kubernetes -o jsonpath='{range .subsets[*].addresses[*]}{.ip}{"\n"}{end}'

kind-operator-patch-api-server-endpoints:
	@ips="$$( $(KUBECTL) get endpoints kubernetes -o jsonpath='{range .subsets[*].addresses[*]}{.ip}{"\n"}{end}' | awk 'NF { printf "%s\"%s\"", sep, $$1; sep="," }' )"; \
	if [ -z "$$ips" ]; then echo "no Kubernetes API endpoint IPs found" >&2; exit 1; fi; \
	$(KUBECTL) -n "$(KIND_OPERATOR_NAMESPACE)" patch openbaocluster "$(KIND_OPERATOR_OPENBAO_CLUSTER)" --type merge --patch "{\"spec\":{\"network\":{\"apiServerEndpointIPs\":[$$ips]}}}"

kind-operator-apply-rules:
	$(KUBECTL) -n "$(KIND_OPERATOR_PROMETHEUS_RULE_NAMESPACE)" apply -f "generated/prometheusrules/$(KIND_OPERATOR_RULE_PROFILE)/openbao-recording-rules.yaml"
	$(KUBECTL) -n "$(KIND_OPERATOR_PROMETHEUS_RULE_NAMESPACE)" apply -f "generated/prometheusrules/$(KIND_OPERATOR_RULE_PROFILE)/openbao-alerts.yaml"
	$(KUBECTL) -n "$(KIND_OPERATOR_PROMETHEUS_RULE_NAMESPACE)" apply -f "generated/prometheusrules/$(KIND_OPERATOR_RULE_PROFILE)/openbao-warning-alerts.yaml"
	$(KUBECTL) -n "$(KIND_OPERATOR_PROMETHEUS_RULE_NAMESPACE)" apply -f "generated/prometheusrules/$(KIND_OPERATOR_RULE_PROFILE)/openbao-security-alerts.yaml"

kind-operator-validate:
	$(KUBECTL) get crd openbaoclusters.openbao.org
	$(KUBECTL) -n "$(KIND_OPERATOR_TENANT_NAMESPACE)" get openbaotenant "$(KIND_OPERATOR_TENANT)"
	$(KUBECTL) -n "$(KIND_OPERATOR_TENANT_NAMESPACE)" wait --for=jsonpath='{.status.provisioned}'=true "openbaotenant/$(KIND_OPERATOR_TENANT)" --timeout="$(KIND_OPERATOR_WAIT_TIMEOUT)"
	$(KUBECTL) -n "$(KIND_OPERATOR_NAMESPACE)" get openbaocluster "$(KIND_OPERATOR_OPENBAO_CLUSTER)"
	$(KUBECTL) -n "$(KIND_OPERATOR_NAMESPACE)" wait --for=condition=Available "openbaocluster/$(KIND_OPERATOR_OPENBAO_CLUSTER)" --timeout="$(KIND_OPERATOR_WAIT_TIMEOUT)"
	$(KUBECTL) -n "$(KIND_OPERATOR_NAMESPACE)" get servicemonitor -l "openbao.org/cluster=$(KIND_OPERATOR_OPENBAO_CLUSTER)"
	$(MAKE) validate-dashboard-queries

kind-operator-down:
	$(KIND) delete cluster --name "$(KIND_OPERATOR_CLUSTER)"

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

test: test-fixtures contracts-verify docs-verify docs-build docs-site-links validate-generated test-unit

verify: generate test
	git diff --exit-code -- generated

verify-live: validate-dashboard-queries

release-artifacts: release-bundle checksums

release-bundle:
	$(GO) run ./cmd/openbao-observability release bundle \
		--version "$(VERSION)" \
		--output "$(RELEASE_BUNDLE)" \
		--source-date-epoch "$(SOURCE_DATE_EPOCH)"

checksums:
	$(GO) run ./cmd/openbao-observability release checksums \
		--dir "$(DIST_DIR)" \
		--output "$(DIST_DIR)/checksums.txt"

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

docs-build:
	$(HUGO_RUN) --source . --baseURL "$(DOCS_BASE_URL)" --destination "$(DOCS_OUT)" --cleanDestinationDir --gc --minify

docs-site-links:
	$(GO) run ./cmd/openbao-observability validate site-links \
		--site-root "$(DOCS_OUT)" \
		--base-url "$(DOCS_BASE_URL)"

docs-serve:
	$(HUGO_RUN) server --source . --baseURL http://localhost:1313/

validate-dashboard-queries:
	$(GO) run ./cmd/openbao-observability validate dashboard-queries \
		--contracts "$(DASHBOARD_CONTRACTS)" \
		--generated "$(GENERATED_DASHBOARDS)" \
		--prometheus-url "$(PROMETHEUS_URL)" \
		--loki-url "$(LOKI_URL)"
