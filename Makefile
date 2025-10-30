SHELL := /bin/bash
BIN_DIR := bin
BINARY := $(BIN_DIR)/pellets
GOBIN := $(CURDIR)/bin

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
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

e2e: build
	go test ./test/e2e -count=1 -parallel=4

docker:
	docker build -t pellets-tracker:latest .

clean:
	rm -rf $(BIN_DIR)

tools:
	go install go.uber.org/mock/mockgen@v0.6.0
