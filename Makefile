# MIT License
#
# Copyright (c) 2026 Nguyen Thanh Phuong
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.


.PHONY: all build server cli run-server generate test test-coverage lint fmt vet tidy vendor clean build-linux package help

# Variables
# Variables
CLI_BINARY=bin/unostat
GO=go
LINT=golangci-lint
DIST_DIR=dist
PACKAGE_NAME=unostat-$(VERSION)-$(shell go env GOOS)-$(shell go env GOARCH)

# Build Information (Inject these into the binary)
# Note: These shell commands assume a Unix-like environment (Git Bash) or compatible Make on Windows.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo unknown)

# Linker Flags: -s -w to strip debug info (smaller binary), -X to inject assertions
LDFLAGS := -ldflags "-s -w -X github.com/phuonguno98/unostat/pkg/version.Version=$(VERSION) -X github.com/phuonguno98/unostat/pkg/version.Commit=$(COMMIT) -X github.com/phuonguno98/unostat/pkg/version.Date=$(DATE)"

# Determine OS for file extensions
ifeq ($(OS),Windows_NT)
    BINARY_EXT=.exe
    RM_CMD=if exist bin rmdir /s /q bin && if exist coverage.out del coverage.out
else
    BINARY_EXT=
    RM_CMD=rm -rf bin $(DIST_DIR) coverage.out
endif

# Targets

all: generate build

build: cli

winres:
ifeq ($(OS),Windows_NT)
	@echo "Generating Windows resources..."
	cd cmd && go-winres make --product-version git-tag --file-version git-tag
endif

cli: winres
	@echo "Building UnoStat (Version: $(VERSION))..."
	$(GO) build $(LDFLAGS) -o $(CLI_BINARY)$(BINARY_EXT) ./cmd


# Cross-compilation for Linux (common for servers)
build-linux:
	@echo "Building binary for Linux..."
	set GOOS=linux&& set GOARCH=amd64&& $(GO) build $(LDFLAGS) -o $(CLI_BINARY)$(BINARY_EXT) ./cmd

# Packaging
package: build
	@echo "Packaging $(PACKAGE_NAME)..."
	@mkdir -p $(DIST_DIR)/$(PACKAGE_NAME)
	@cp $(CLI_BINARY)$(BINARY_EXT) $(DIST_DIR)/$(PACKAGE_NAME)/
	@cp README.md LICENSE CHANGELOG.md $(DIST_DIR)/$(PACKAGE_NAME)/
	@if [ -d "docs" ]; then cp -r docs $(DIST_DIR)/$(PACKAGE_NAME)/; fi
	@tar -C $(DIST_DIR) -czvf $(DIST_DIR)/$(PACKAGE_NAME).tar.gz $(PACKAGE_NAME)
	@echo "Package created: $(DIST_DIR)/$(PACKAGE_NAME).tar.gz"

# Development helpers
run-server:
	@echo "Running visualize locally..."
	$(GO) run $(LDFLAGS) ./cmd visualize

generate:
	@echo "Running go generate..."
	$(GO) generate ./...

# Quality Control
test:
	@echo "Running tests..."
	$(GO) test -v -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

lint:
	@echo "Running linter..."
	$(LINT) run

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

vet:
	@echo "Vetting code..."
	$(GO) vet ./...

# Module Management
tidy:
	@echo "Tidying module dependencies..."
	$(GO) mod tidy

vendor:
	@echo "Vendoring dependencies..."
	$(GO) mod vendor

download:
	@echo "Downloading dependencies..."
	$(GO) mod download

# Cleanup
clean:
	@echo "Cleaning up..."
	$(GO) clean
	-$(RM_CMD)

help:
	@echo "UnoStat Makefile (Windows/Linux compatible)"
	@echo ""
	@echo "Build Targets:"
	@echo "  all             Generate code and build binaries (default)"
	@echo "  build           Build server and CLI"
	@echo "  server          Build server binary only"
	@echo "  cli             Build CLI binary only"
	@echo "  build-linux     Cross-compile binaries for Linux (AMD64)"
	@echo ""
	@echo "Development:"
	@echo "  run-server      Run the server directly"
	@echo "  generate        Run go generate"
	@echo "  fmt             Format code"
	@echo ""
	@echo "Quality Control:"
	@echo "  test            Run unit tests"
	@echo "  test-coverage   Run tests & show coverage report"
	@echo "  lint            Run golangci-lint"
	@echo ""
	@echo "Project Management:"
	@echo "  tidy            Go mod tidy"
	@echo "  vendor          Go mod vendor"
	@echo "  package         Package binary and docs into tar.gz"
	@echo "  clean           Remove artifacts"
