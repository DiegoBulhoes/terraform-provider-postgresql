.PHONY: build test lint clean install fmt vet testacc testacc-cover cover-html docs docs-validate

BINARY_NAME=terraform-provider-postgresql
GO=go
GOFLAGS=-v

build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .

install:
	$(GO) install .

test:
	$(GO) test -v -race -coverprofile=coverage.out ./...

testacc:
	TF_ACC=1 $(GO) test -v -timeout 600s -parallel 1 ./internal/provider/

testacc-cover:
	TF_ACC=1 $(GO) test -timeout 600s -parallel 1 -coverprofile=coverage.out ./internal/provider/
	$(GO) tool cover -func=coverage.out | tail -1

cover-html: coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser"

fmt:
	$(GO) fmt ./...
	goimports -w .

vet:
	$(GO) vet ./...

lint: vet
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME) coverage.out

tidy:
	$(GO) mod tidy

docs:
	tfplugindocs generate

docs-validate:
	tfplugindocs validate
