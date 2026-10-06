# whereismyspace — build & dev tasks

BINARY      := whereismyspace
PKG         := .
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DIR   := bin
LDFLAGS     := -s -w -X main.version=$(VERSION)

# Platforms for `make release` (GOOS/GOARCH pairs).
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the binary into ./$(BINARY)
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) $(PKG)

.PHONY: install
install: ## Install the binary into $GOBIN / $GOPATH/bin
	go install -ldflags '$(LDFLAGS)' $(PKG)

.PHONY: run
run: ## Build and run against the current directory (ARGS="..." to pass flags)
	go run $(PKG) $(ARGS)

.PHONY: test
test: ## Run the test suite
	go test ./...

.PHONY: race
race: ## Run tests under the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run tests and open an HTML coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format all Go source
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofmt-clean
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

.PHONY: tidy
tidy: ## Tidy the module
	go mod tidy

.PHONY: check
check: fmt-check vet race ## Run format check, vet, and race tests

.PHONY: release
release: ## Cross-compile release binaries into $(BUILD_DIR)/
	@mkdir -p $(BUILD_DIR)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=; [ "$$os" = windows ] && ext=.exe; \
		out=$(BUILD_DIR)/$(BINARY)-$$os-$$arch$$ext; \
		echo "  building $$out"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags '$(LDFLAGS)' -o $$out $(PKG) || exit 1; \
	done

.PHONY: package-release
package-release: ## Build versioned release archives and SHA256SUMS (VERSION=v1.0.0)
	python3 scripts/release.py $(VERSION)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BINARY) $(BUILD_DIR) coverage.out

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
