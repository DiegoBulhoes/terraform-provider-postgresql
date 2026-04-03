.PHONY: build test lint clean install fmt vet testacc testacc-cover cover-html docs changelog tidy

BINARY_NAME=terraform-provider-postgresql
GO=go
GOFLAGS=-v

build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .

install:
	$(GO) install .

test:
	$(GO) test -v -race ./...

testacc:
	TF_ACC=1 $(GO) test -tags integration -v -timeout 600s ./...

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

changelog:
	git-cliff -o CHANGELOG.md
