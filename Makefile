# Meerkat dev tasks.
.PHONY: help build test vet fmt lint race install scan all clean release-local

BIN     := meerkat
PKG     := github.com/ujjwalredd/meerkat
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)

help: ## show this help
	@awk 'BEGIN{FS=":.*##"; printf "make <target>\n\n"} /^[a-zA-Z_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## build the meerkat binary into ./bin
	@mkdir -p bin
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BIN) ./cmd/meerkat

install: ## install meerkat into $$GOBIN (or $$GOPATH/bin)
	go install -trimpath -ldflags '$(LDFLAGS)' ./cmd/meerkat

test: ## run unit tests
	go test ./...

race: ## run tests with the race detector
	go test -race ./...

vet: ## go vet
	go vet ./...

fmt: ## gofmt -w
	gofmt -w .

lint: vet ## vet + gofmt check
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

scan: build ## self-scan repo with built binary
	./bin/$(BIN) policy validate
	./bin/$(BIN) scan .

all: lint race build ## lint + race + build

clean: ## remove build artifacts
	rm -rf bin dist

release-local: ## cross-build release binaries into ./dist (no upload)
	@mkdir -p dist
	@for goos in darwin linux windows; do \
	  for goarch in amd64 arm64; do \
	    [ "$$goos" = "windows" ] && [ "$$goarch" = "arm64" ] && continue; \
	    ext=""; [ "$$goos" = "windows" ] && ext=".exe"; \
	    echo "==> $$goos/$$goarch"; \
	    CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -trimpath -ldflags '$(LDFLAGS)' \
	      -o "dist/$(BIN)-$$goos-$$goarch$$ext" ./cmd/meerkat; \
	  done; \
	done
	@ls -la dist
