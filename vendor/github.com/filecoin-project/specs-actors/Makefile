GO_BIN ?= go
GOLINT ?= golangci-lint

all: build lint test tidy
.PHONY: all

build:
	$(GO_BIN) build ./...
.PHONY: build

test:
	$(GO_BIN) test ./...
.PHONY: test

test-coverage:
	$(GO_BIN) test -coverprofile=coverage.out ./...
.PHONY: test-coverage

tidy:
	$(GO_BIN) mod tidy
.PHONY: tidy

lint:
	$(GOLINT) run --skip-dirs-use-default=false ./...
.PHONY: lint

gen:
	$(GO_BIN) run ./gen/gen.go
.PHONY: gen
