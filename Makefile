BIN := slk
PKG := ./cmd/slk
GO  := go

.PHONY: help build test fmt vet tidy install clean run

help: ## show targets
	@awk 'BEGIN{FS=":.*##"}/^[a-zA-Z_-]+:.*##/{printf "  %-10s %s\n",$$1,$$2}' $(MAKEFILE_LIST)

build: ## build slk binary
	$(GO) build -o $(BIN) $(PKG)

test: ## run all tests
	$(GO) test ./... -count=1

fmt: ## go fmt all
	$(GO) fmt ./...

vet: ## go vet all
	$(GO) vet ./...

tidy: ## tidy modules
	$(GO) mod tidy

install: ## install to GOBIN
	$(GO) install $(PKG)

clean: ## remove build artifacts
	rm -f $(BIN)

run: build ## build and run
	./$(BIN)
