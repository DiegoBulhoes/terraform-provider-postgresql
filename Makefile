.PHONY: build test lint clean install fmt vet testacc testacc-cover cover-html docs tidy

BINARY_NAME=terraform-provider-postgresql
GO=go
GOFLAGS=-v

build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .

install:
	$(GO) install .

test:
	$(GO) test -v -race ./...

PG_VERSIONS ?= 14 15 16 17

testacc:
	@for v in $(PG_VERSIONS); do \
		echo "=== PostgreSQL $$v ==="; \
		POSTGRES_IMAGE=postgres:$$v-alpine TF_ACC=1 TF_ACC_TERRAFORM_PATH=$$(which terraform) $(GO) test -tags integration -v -timeout 600s -count=1 ./... || exit 1; \
		echo ""; \
	done

testacc-cover:
	TF_ACC=1 $(GO) test -tags integration -timeout 600s -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

cover-html: coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser"

fmt:
	$(GO) fmt ./...
	$(GO) tool goimports -w .

vet:
	$(GO) vet ./...

lint: vet
	$(GO) tool golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html

tidy:
	$(GO) mod tidy

docs:
	$(GO) tool tfplugindocs validate
	$(GO) tool tfplugindocs generate
