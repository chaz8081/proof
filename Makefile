BINARY := proof
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/chaz8081/proof/internal/cli.Version=$(VERSION) \
           -X github.com/chaz8081/proof/internal/cli.Commit=$(COMMIT) \
           -X github.com/chaz8081/proof/internal/cli.BuildDate=$(DATE)

.PHONY: build install clean test

## build: Build the binary with Copilot SDK support
build:
	go build -tags=copilot -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/proof

## install: Build and install to /usr/local/bin
install: build
	sudo mv $(BINARY) /usr/local/bin/$(BINARY)
	@echo "✓ proof $(VERSION) installed to /usr/local/bin/proof"

## install-local: Build and install to ~/bin (no sudo)
install-local: build
	mkdir -p $(HOME)/bin
	mv $(BINARY) $(HOME)/bin/$(BINARY)
	@echo "✓ proof $(VERSION) installed to ~/bin/proof"

## test: Run all tests
test:
	go test ./... -count=1

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)

## help: Show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
