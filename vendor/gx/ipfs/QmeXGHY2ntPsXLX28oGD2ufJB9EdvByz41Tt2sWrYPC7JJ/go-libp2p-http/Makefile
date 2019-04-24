all: deps
clean: rwundo
gx:
	go get github.com/whyrusleeping/gx
	go get github.com/whyrusleeping/gx-go
deps: gx
	gx --verbose install --global
	gx-go rewrite
test: deps
	go test -v -race -covermode=atomic -coverprofile=coverage.txt .
rw:
	gx-go rewrite
rwundo:
	gx-go rewrite --undo
publish: rwundo
	gx publish
.PHONY: all gx deps test rw rwundo publish clean
