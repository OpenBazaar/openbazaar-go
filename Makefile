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

DOCKER_QA_PROFILE ?= docker.dev.ob1.io
DOCKER_QA_VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
DOCKER_QA_IMAGE_NAME ?= $(DOCKER_QA_PROFILE)/openbazaar-qa:$(DOCKER_QA_VERSION)

.PHONY: docker
docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: push_docker
push_docker:
	docker push $(DOCKER_IMAGE_NAME)

.PHONY: qa_docker
qa_docker:
	docker build -t $(DOCKER_QA_IMAGE_NAME) .

.PHONY: push_qa_docker
push_qa_docker:
	docker push $(DOCKER_QA_IMAGE_NAME)