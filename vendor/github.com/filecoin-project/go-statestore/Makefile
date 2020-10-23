SHELL=/usr/bin/env bash

all: build
.PHONY: all

test:
	go test -v $(GOFLAGS) ./...
.PHONY: test

lint:
	golangci-lint run -v  --concurrency 2 --new-from-rev origin/master
.PHONY: lint

build:
	go build -v $(GOFLAGS) ./...
.PHONY: build
