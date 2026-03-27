GO := $(shell find $(HOME)/.local/share/mise/installs/go -name go -type f 2>/dev/null | sort | tail -1)
ifeq ($(GO),)
GO := go
endif

BIN := bnn

.PHONY: test test/dsl test/visitor test/cmd build run fmt vet debug clean

## build: compile to ./bnn
build:
	$(GO) build -o $(BIN) .

## run: run without building (go run .)
run:
	$(GO) run . $(ARGS)

## test: run all tests
test:
	$(GO) test ./...

## test/dsl: parser + lexer tests (verbose)
test/dsl:
	$(GO) test ./internal/parser/dsl/... -v

## test/visitor: validator, resolver, dryrun, execute tests (verbose)
test/visitor:
	$(GO) test ./visitor/... -v

## test/cmd: cobra CLI tests (verbose)
test/cmd:
	$(GO) test ./cmd/... -v

## debug: run apply --dry with debug logging
debug: build
	BNN_DEBUG=1 ./$(BIN) apply --dry

## fmt: format all source
fmt:
	$(GO) fmt ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## clean: remove built binary
clean:
	rm -f $(BIN)

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  make /'
