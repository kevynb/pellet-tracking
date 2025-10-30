SHELL := /bin/bash
BIN_DIR := bin
BINARY := $(BIN_DIR)/pellets
GOBIN := $(CURDIR)/bin
GOLANGCI_LINT_VERSION := 2.6.0
GOLANGCI_LINT_TAG := v$(GOLANGCI_LINT_VERSION)
GOLANGCI_LINT_ARCHIVE := https://github.com/golangci/golangci-lint/releases/download/$(GOLANGCI_LINT_TAG)/golangci-lint-$(GOLANGCI_LINT_VERSION)-linux-amd64.tar.gz

export GOBIN
export PATH := $(GOBIN):$(PATH)

.PHONY: build test lint e2e run docker clean tools

build: $(BINARY)

$(BINARY):
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/app

run: build
	$(BINARY)

test:
	go test ./...

lint: $(GOBIN)/golangci-lint
	golangci-lint run ./...

$(GOBIN)/golangci-lint:
	@mkdir -p $(GOBIN)
	tmp_dir=$$(mktemp -d); \
		curl -sSL $(GOLANGCI_LINT_ARCHIVE) -o $$tmp_dir/golangci-lint.tar.gz; \
		tar -xzf $$tmp_dir/golangci-lint.tar.gz -C $$tmp_dir; \
		install -m 0755 $$tmp_dir/golangci-lint-$(GOLANGCI_LINT_VERSION)-linux-amd64/golangci-lint $(GOBIN)/golangci-lint; \
		rm -rf $$tmp_dir

e2e: build
	go test ./test/e2e -count=1 -parallel=4

docker:
	docker build -t pellets-tracker:latest .

clean:
	rm -rf $(BIN_DIR)

tools:
	go install go.uber.org/mock/mockgen@v0.6.0
