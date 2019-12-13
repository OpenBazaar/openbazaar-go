GIT_SHA ?= $(shell git rev-parse --short=8 HEAD)
GIT_TAG ?= $(shell git describe --tags --abbrev=0)

##
## Building
##

.PHONY: ios_framework
ios_framework:
	gomobile bind -target=ios github.com/OpenBazaar/openbazaar-go/mobile

.PHONY: android_framework
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
## Testing
##

OPENBAZAARD_NAME ?= openbazaard-$(GIT_TAG)-$(GIT_SHA)
BITCOIND_PATH ?= .

.PHONY: openbazaard
openbazaard:
	$(info "Building openbazaar daemon...")
	go build -o ./$(OPENBAZAARD_NAME) .

.PHONY: qa
qa: openbazaard
	$(info "Running QA... (openbazaard: ../$(OPENBAZAARD_NAME) bitcoind: $(BITCOIND_PATH)/bin/bitcoind)")
	(cd qa && ./runtests.sh ../$(OPENBAZAARD_NAME) $(BITCOIND_PATH)/bin/bitcoind)

##
## Docker
##
PUBLIC_DOCKER_REGISTRY ?= openbazaar
OB_DOCKER_REGISTRY ?= docker.dev.ob1.io

DOCKER_IMAGE_NAME ?= $(PUBLIC_DOCKER_REGISTRY_PROFILE)/server:$(GIT_TAG)
DOCKER_QA_IMAGE_NAME ?= $(OB_DOCKER_REGISTRY)/openbazaar-qa:$(GIT_SHA)
DOCKER_DEV_IMAGE_NAME ?= $(OB_DOCKER_REGISTRY)/openbazaar-dev:$(GIT_SHA)


.PHONY: docker
docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: push_docker
push_docker:
	docker push $(DOCKER_IMAGE_NAME)

.PHONY: qa_docker
qa_docker:
	docker build -t $(DOCKER_QA_IMAGE_NAME) -f ./Dockerfile.qa .

.PHONY: push_qa_docker
push_qa_docker:
	docker push $(DOCKER_QA_IMAGE_NAME)

.PHONY: dev_docker
dev_docker:
	docker build -t $(DOCKER_DEV_IMAGE_NAME) -f ./Dockerfile.dev .
