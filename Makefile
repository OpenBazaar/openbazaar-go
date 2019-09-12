##
## Building
##

ios_framework:
	gomobile bind -target=ios github.com/OpenBazaar/openbazaar-go/mobile

android_framework:
	gomobile bind -target=android github.com/OpenBazaar/openbazaar-go/mobile

##
## Protobuf compilation
##
P_TIMESTAMP = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp
P_ANY = Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

PKGMAP = $(P_TIMESTAMP),$(P_ANY)

.PHONY: protos
protos:
	cd pb/protos && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=$(PKGMAP):.. *.proto

##
## Docker
##
DOCKER_PROFILE ?= openbazaar
DOCKER_VERSION ?= $(shell git describe --tags --abbrev=0)
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/server:$(DOCKER_VERSION)

.PHONY: docker
docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: push_docker
push_docker:
	docker push $(DOCKER_IMAGE_NAME)

.PHONY: qa
qa:
	go build -o ./openbazaar-qa ./openbazaard.go
	(cd qa && ./runtests.sh ../openbazaar-qa /opt/bitcoin-0.16.3/bin/bitcoind)
	rm ./openbazaar-qa
