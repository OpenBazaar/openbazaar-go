SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

BINARY=openbazaard

VERSION=0.2.0

LDFLAGS=-ldflags "-X github.com/OpenBazaar/openbazaar-go/main.Version=${VERSION}"

.DEFAULT_GOAL: $(BINARY)

$(BINARY): $(SOURCES)
	go build ${LDFLAGS} -o ${BINARY} main.go

godep:
	go get github.com/tools/godep

toolkit_upgrade: gx_upgrade gxgo_upgrade godep

gx_upgrade:
	go get -u github.com/whyrusleeping/gx

gxgo_upgrade:
	go get -u github.com/whyrusleeping/gx-go

deps: 
	gx --verbose install --global

vendor:
	godep save

.PHONY: install
install: deps
	godep go install ${LDFLAGS} ./...

.PHONY: clean
clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi
	go clean -i -ldflags=$(LDFLAGS)
