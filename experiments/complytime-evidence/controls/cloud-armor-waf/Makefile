SHELL := /bin/bash

override CONTROL_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
override BIN_DIR := $(CONTROL_DIR)/bin
override GO_PATH := $(CONTROL_DIR)/.complytime/gopath
override GO_CACHE := $(CONTROL_DIR)/.complytime/go-cache
COMPLYCTL_VERSION := v1.0.0
OPA_PROVIDER_VERSION := v0.2.1
COMPLYPACK_VERSION := v0.0.8
export PROJECT BACKEND_SERVICE SECURITY_POLICY

.PHONY: install-tools test scan-fixture scan-live clean

install-tools:
	@mkdir -p "$(BIN_DIR)" "$(GO_PATH)" "$(GO_CACHE)"
	@GOBIN="$(BIN_DIR)" GOPATH="$(GO_PATH)" GOCACHE="$(GO_CACHE)" GOTOOLCHAIN=auto \
		go install github.com/complytime/complyctl/cmd/complyctl@$(COMPLYCTL_VERSION)
	@GOBIN="$(BIN_DIR)" GOPATH="$(GO_PATH)" GOCACHE="$(GO_CACHE)" GOTOOLCHAIN=auto \
		go install github.com/complytime/complytime-providers/cmd/opa-provider@$(OPA_PROVIDER_VERSION)
	@GOBIN="$(BIN_DIR)" GOPATH="$(GO_PATH)" GOCACHE="$(GO_CACHE)" GOTOOLCHAIN=auto \
		go install github.com/complytime/complypack/cmd/complypack@$(COMPLYPACK_VERSION)
	@mv "$(BIN_DIR)/opa-provider" "$(BIN_DIR)/complyctl-provider-opa"
	@printf 'Installed pinned complyctl, OPA provider, and CompliPack under %s\n' "$(BIN_DIR)"

test:
	@bash tests/test-clean-scope.sh
	@bash tests/test-evaluation-log.sh
	@bash tests/test-live-collector.sh
	@bash tests/test-normalizer.sh
	@bash tests/run-fixtures.sh
	@bash tests/test-complytime-fixture.sh

scan-fixture:
	@bash scripts/run-fixture-scans.sh

scan-live:
	@bash scripts/run-live-check.sh \
		--project "$$PROJECT" \
		--backend-service "$$BACKEND_SERVICE" \
		--security-policy "$$SECURITY_POLICY"

clean:
	@rm -rf "$(BIN_DIR)" "$(CONTROL_DIR)/.complytime"
	@printf 'Removed local WAF prototype build and scan outputs.\n'
