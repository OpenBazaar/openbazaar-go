export GO111MODULE ?= on

all: export GO111MODULE := off
all:
	go get -u golang.org/x/tools/cmd/goimports
	go get -u golang.org/x/lint/golint

fix:
	@goimports -w *.go

lint:
	@! goimports -d . | grep -vF "no errors"
	@golint -set_exit_status ./...

bench:
	@go test -v -run=^$$ -bench=.

test: lint
	@go test -v ./...
