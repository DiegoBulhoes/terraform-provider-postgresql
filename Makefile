.DEFAULT_GOAL := all

BINARY_NAME = terraform-provider-postgresql
GO          = go
PG_VERSIONS ?= 14 15 16 17

## ── All (CI pipeline) ───────────────────────────────────────────
.PHONY: all
all: tidy lint security test build docs

## ── Build & Install ─────────────────────────────────────────────
.PHONY: build
build:
	$(GO) build -v -o $(BINARY_NAME) .

.PHONY: install
install:
	$(GO) install .

## ── Quality ─────────────────────────────────────────────────────
.PHONY: fmt
fmt:
	$(GO) fmt ./...
	$(GO) tool goimports -w .

.PHONY: lint
lint: fmt
	$(GO) vet ./...
	$(GO) tool golangci-lint run ./...

.PHONY: security
security:
	govulncheck ./...

## ── Tests ───────────────────────────────────────────────────────
.PHONY: test
test:
	$(GO) test -race -count=1 -coverprofile=coverage.out -covermode=atomic -coverpkg=./internal/... ./test/unit/...
	$(GO) tool cover -func=coverage.out | tail -1
	@for v in $(PG_VERSIONS); do \
		echo "=== PostgreSQL $$v ==="; \
		POSTGRES_IMAGE=postgres:$$v-alpine TF_ACC=1 \
		TF_ACC_TERRAFORM_PATH=$$(which terraform) \
		$(GO) test -tags integration -timeout 600s -count=1 ./test/integration/... || exit 1; \
		echo ""; \
	done
	$(GO) tool cover -html=coverage.out -o coverage.html

## ── Docs ────────────────────────────────────────────────────────
.PHONY: docs
docs:
	$(GO) tool tfplugindocs generate
	$(GO) tool tfplugindocs validate

## ── CI parity (matches .github/workflows/test.yml) ──────────────
# `make ci` mirrors the GitHub Actions `checks` + `unit` jobs and does
# NOT modify files (unlike `make lint`, which runs `go fmt` first).
# Run this before pushing to catch CI failures locally.
#
# `make ci-acceptance` adds the acceptance-tests matrix against PG 14-17
# (needs Docker + Terraform on PATH).
.PHONY: ci-vet
ci-vet:
	$(GO) vet ./...

.PHONY: ci-fmt-check
ci-fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "ERROR: the following files need formatting (run 'make fmt'):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

.PHONY: ci-lint
ci-lint:
	$(GO) tool golangci-lint run ./...

.PHONY: ci-docs
ci-docs:
	$(GO) tool tfplugindocs validate

.PHONY: ci-vuln
ci-vuln:
	@command -v govulncheck >/dev/null 2>&1 || { \
		echo "installing govulncheck..."; \
		$(GO) install golang.org/x/vuln/cmd/govulncheck@latest; \
	}
	govulncheck ./...

.PHONY: ci-unit
ci-unit:
	$(GO) test -race -count=1 -coverpkg=./internal/... -coverprofile=coverage-unit.out ./test/unit/...
	@$(GO) tool cover -func=coverage-unit.out | tail -1

.PHONY: ci
ci: ci-vet ci-fmt-check ci-lint ci-docs ci-vuln ci-unit
	@echo
	@echo "✓ CI checks + unit tests pass (mirrors .github/workflows/test.yml 'checks' + 'unit' jobs)"
	@echo "  Acceptance tests require Docker + PG — run 'make ci-acceptance' for full parity."

.PHONY: ci-acceptance
ci-acceptance:
	@for v in $(PG_VERSIONS); do \
		echo "=== PostgreSQL $$v ==="; \
		POSTGRES_IMAGE=postgres:$$v-alpine TF_ACC=1 \
		TF_ACC_TERRAFORM_PATH=$$(which terraform) \
		$(GO) test -tags integration -timeout 600s -count=1 -coverpkg=./internal/... ./test/integration/... || exit 1; \
		echo ""; \
	done

## ── Housekeeping ────────────────────────────────────────────────
.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html coverage-unit.out
