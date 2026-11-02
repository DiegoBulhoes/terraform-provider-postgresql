.DEFAULT_GOAL := all

BINARY_NAME = terraform-provider-postgresql
GO          = go
PG_VERSIONS ?= 14 15 16 17 18

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
	$(GO) tool govulncheck ./...

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
	$(GO) tool tfplugindocs generate --provider-name postgresql
	$(GO) tool tfplugindocs validate --provider-name postgresql

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
	$(GO) vet -tags integration ./...

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
	$(GO) tool tfplugindocs validate --provider-name postgresql

.PHONY: ci-vuln
ci-vuln:
	$(GO) tool govulncheck ./...

.PHONY: ci-tidy
ci-tidy:
	$(GO) mod tidy -diff

.PHONY: ci-actions
ci-actions:
	$(GO) tool actionlint

.PHONY: ci-unit
ci-unit:
	$(GO) test -race -count=1 -covermode=atomic -coverpkg=./internal/... -coverprofile=coverage-unit.out ./test/unit/...
	@$(GO) tool cover -func=coverage-unit.out | tail -1

.PHONY: ci
ci: ci-tidy ci-vet ci-fmt-check ci-lint ci-actions ci-docs ci-vuln ci-unit
	@echo
	@echo "✓ CI checks + unit tests pass (mirrors the 'checks' and 'unit' jobs of .github/workflows/ci.yml)"
	@echo "  Acceptance tests require Docker + PG — run 'make ci-acceptance' for full parity."

.PHONY: ci-acceptance
ci-acceptance:
	@for v in $(PG_VERSIONS); do \
		echo "=== PostgreSQL $$v ==="; \
		POSTGRES_IMAGE=postgres:$$v-alpine TF_ACC=1 \
		TF_ACC_TERRAFORM_PATH=$$(which terraform) \
		$(GO) test -tags integration -timeout 900s -count=1 -covermode=atomic -coverpkg=./internal/... \
			-coverprofile=coverage-acceptance-$$v.out ./test/integration/... || exit 1; \
		echo ""; \
	done

# Unit and acceptance coverage together, against the floor CI enforces.
.PHONY: coverage
coverage: ci-unit ci-acceptance
	./scripts/coverage.sh 90 coverage-unit.out coverage-acceptance-17.out

## ── Housekeeping ────────────────────────────────────────────────
.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY_NAME) coverage*.out coverage.html
