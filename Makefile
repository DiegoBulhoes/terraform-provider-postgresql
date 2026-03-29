.PHONY: build test lint clean install fmt vet

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
	TF_ACC=1 $(GO) test -v -race -timeout 120m ./...

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
