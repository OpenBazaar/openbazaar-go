all: examples

examples: deps example blockhandler/blockhandler

blockhandler/blockhandler:
	cd blockhandler && go build blockhandler.go
example:
	go build example.go

gx:
	go get github.com/whyrusleeping/gx
	go get github.com/whyrusleeping/gx-go

deps: gx
	gx --verbose install --global
	gx-go rewrite

publish: gx
	gx-go rewrite --undo
	rm example
	rm blockhandler/blockhandler

.PHONY = publish deps gx-bins examples all
