GO := $(shell find $(HOME)/.local/share/mise/installs/go -name go -type f 2>/dev/null | sort | tail -1)
ifeq ($(GO),)
GO := go
endif

.PHONY: test test/dsl build fmt vet

test:
	$(GO) test ./...

test/dsl:
	$(GO) test ./internal/parser/dsl/... -v

build:
	$(GO) build ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...
