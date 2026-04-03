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

## ── Housekeeping ────────────────────────────────────────────────
.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html
