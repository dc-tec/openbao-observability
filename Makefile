OPENBAO_VERSION ?= 2.5.4
OPENBAO_IMAGE ?= quay.io/openbao/openbao:$(OPENBAO_VERSION)
OPENBAO_PORT_BASE ?= 18220
OPENBAO_ROOT_TOKEN ?= root
FIXTURE_DIR ?= fixtures/captured/openbao-$(OPENBAO_VERSION)
GO ?= go

.PHONY: contracts-verify fixtures-openbao generate test test-fixtures test-unit

contracts-verify:
	$(GO) run ./cmd/openbao-observability contracts verify \
		--contract "contracts/metrics/openbao-core.yaml" \
		--fixtures "$(FIXTURE_DIR)"

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
		--output "generated/prometheusrules/openbao-recording-rules.yaml"

test: test-fixtures contracts-verify test-unit

test-unit:
	$(GO) test ./...

test-fixtures:
	$(GO) run ./cmd/openbao-observability fixtures verify \
		--version "$(OPENBAO_VERSION)" \
		--dir "$(FIXTURE_DIR)"
