# CoolLEDUX Controller — build, test, and deploy targets.
#
# Conventions:
#   - Unit tests are plain `go test ./...` — hermetic, no hardware needed.
#   - Integration tests live behind the `integration` build tag so they are
#     skipped by default. Each integration suite ("ble", "mqtt", "rest") has
#     its own subtag so you can run them independently:
#
#         //go:build integration && ble_hw
#         //go:build integration && mqtt
#         //go:build integration && rest
#
#     Put those files under test/integration/<suite>/ (create on first use).
#
#   - Env vars the integration targets expect (set in your shell before
#     invoking, or export in a .env and `source` it):
#
#         COOLLEDUX_TEST_MAC      BLE MAC of a live panel (for ble_hw)
#         COOLLEDUX_TEST_BROKER   MQTT broker URL        (for mqtt)
#         COOLLEDUX_TEST_API      base URL of a running service (for rest)

SHELL := /usr/bin/env bash
GO ?= go
BINARY := coolledux-controller
PKG := ./...
BUILD_DIR := .
BUILD_TAGS ?=
COVER_PROFILE := coverage.out
COVER_HTML := coverage.html

# Integration-test env vars with sensible defaults for local dev.
COOLLEDUX_TEST_MAC    ?= 01:00:00:FB:A4:16
COOLLEDUX_TEST_BROKER ?= tcp://localhost:1883
COOLLEDUX_TEST_API    ?= http://localhost:8080

# Default target prints help.
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---------- Build & housekeeping ----------

.PHONY: build
build: ## Build the coolledux-controller binary.
	$(GO) build -o $(BUILD_DIR)/$(BINARY) ./cmd/coolledux-controller

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

.PHONY: fmt
fmt: ## gofmt -s -w over the tree.
	$(GO) fmt $(PKG)

.PHONY: vet
vet: ## go vet over the tree.
	$(GO) vet $(PKG)

.PHONY: clean
clean: ## Remove build + coverage artifacts.
	rm -f $(BINARY) $(COVER_PROFILE) $(COVER_HTML)

.PHONY: extract-assets
extract-assets: ## Extract APK assets required for //go:embed (one-time).
	./scripts/extract-assets.sh

# ---------- Unit tests ----------
#
# These must stay hermetic. No BLE, no MQTT broker, no running service.

.PHONY: test
test: test-unit ## Alias for test-unit.

.PHONY: test-unit
test-unit: ## Run unit tests (no external deps).
	$(GO) test -count=1 $(PKG)

.PHONY: test-short
test-short: ## Run unit tests in short mode (skip slow tests).
	$(GO) test -short -count=1 $(PKG)

.PHONY: test-race
test-race: ## Run unit tests with the race detector (excludes internal/ble due to upstream races in tinygo.org/x/bluetooth).
	$(GO) test -race -count=1 $$($(GO) list $(PKG) | grep -v '/internal/ble$$')

.PHONY: cover
cover: ## Unit test coverage summary.
	$(GO) test -covermode=atomic -coverprofile=$(COVER_PROFILE) $(PKG)
	$(GO) tool cover -func=$(COVER_PROFILE) | tail -1

.PHONY: cover-html
cover-html: cover ## Generate an HTML coverage report.
	$(GO) tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)
	@echo "coverage report: $(COVER_HTML)"

# ---------- Integration tests ----------
#
# Gated behind the `integration` build tag plus a per-suite subtag so unit-test
# runs never touch hardware or a broker. Create files like:
#
#     test/integration/ble/<name>_test.go   with //go:build integration && ble_hw
#     test/integration/mqtt/<name>_test.go  with //go:build integration && mqtt
#     test/integration/rest/<name>_test.go  with //go:build integration && rest
#
# The suite directories are empty until someone writes the first test; these
# targets are ready to pick them up once they exist.

.PHONY: test-integration
test-integration: test-integration-ble test-integration-mqtt test-integration-rest ## Run every integration suite.

.PHONY: test-integration-ble
test-integration-ble: ## Integration tests against a real panel (requires COOLLEDUX_TEST_MAC).
	@if [ -z "$(COOLLEDUX_TEST_MAC)" ]; then echo "COOLLEDUX_TEST_MAC not set"; exit 1; fi
	COOLLEDUX_TEST_MAC=$(COOLLEDUX_TEST_MAC) \
		$(GO) test -tags "integration ble_hw" -count=1 -timeout 5m ./test/integration/ble/...

.PHONY: test-integration-mqtt
test-integration-mqtt: ## Integration tests against a live MQTT broker (requires COOLLEDUX_TEST_BROKER).
	@if [ -z "$(COOLLEDUX_TEST_BROKER)" ]; then echo "COOLLEDUX_TEST_BROKER not set"; exit 1; fi
	COOLLEDUX_TEST_BROKER=$(COOLLEDUX_TEST_BROKER) \
		$(GO) test -tags "integration mqtt" -count=1 -timeout 2m ./test/integration/mqtt/...

.PHONY: test-integration-rest
test-integration-rest: ## Integration tests against a running REST API (requires COOLLEDUX_TEST_API).
	@if [ -z "$(COOLLEDUX_TEST_API)" ]; then echo "COOLLEDUX_TEST_API not set"; exit 1; fi
	COOLLEDUX_TEST_API=$(COOLLEDUX_TEST_API) \
		$(GO) test -tags "integration rest" -count=1 -timeout 2m ./test/integration/rest/...

.PHONY: test-all
test-all: test-unit test-integration ## Unit + every integration suite.

# ---------- Run & Docker ----------

.PHONY: run
run: build ## Build and run with config.yaml in the foreground.
	./$(BINARY) --config config.yaml

.PHONY: docker-build
docker-build: ## Build the Docker image.
	docker compose build

.PHONY: docker-up
docker-up: ## Start the service and Mosquitto broker.
	docker compose up -d

.PHONY: docker-down
docker-down: ## Stop the service and Mosquitto broker.
	docker compose down

.PHONY: docker-logs
docker-logs: ## Tail service logs.
	docker compose logs -f coolledux-controller
