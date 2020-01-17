.DEFAULT_GOAL := help

##
## Global ENV vars
##

GIT_SHA ?= $(shell git rev-parse --short=8 HEAD)
GIT_TAG ?= $(shell git describe --tags --abbrev=0)

##
## Helpful Help
##

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'


##
## Building
##

.PHONY: ios_framework
ios_framework: ## Build iOS Framework for mobile
	gomobile bind -target=ios github.com/OpenBazaar/openbazaar-go/mobile

.PHONY: android_framework
android_framework: ## Build Android Framework for mobile
	gomobile bind -target=android github.com/OpenBazaar/openbazaar-go/mobile

##
## Protobuf compilation
##
P_TIMESTAMP = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp
P_ANY = Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

PKGMAP = $(P_TIMESTAMP),$(P_ANY)

.PHONY: protos
protos: ## Build go files for proto definitions
	cd pb/protos && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=$(PKGMAP):.. *.proto


##
## Testing
##
OPENBAZAARD_NAME ?= openbazaard-$(GIT_TAG)-$(GIT_SHA)
BITCOIND_PATH ?= .

.PHONY: openbazaard
openbazaard: ## Build daemon
	$(info "Building openbazaar daemon...")
	go build -o ./$(OPENBAZAARD_NAME) .

.PHONY: qa_test
qa_test: openbazaard ## Run QA test suite against current working copy
	$(info "Running QA... (openbazaard: ../$(OPENBAZAARD_NAME) bitcoind: $(BITCOIND_PATH)/bin/bitcoind)")
	(cd qa && ./runtests.sh ../$(OPENBAZAARD_NAME) $(BITCOIND_PATH)/bin/bitcoind)

##
## Docker
##
PUBLIC_DOCKER_REGISTRY ?= openbazaar
QA_DEV_TAG ?= 0.10

DOCKER_SERVER_IMAGE_NAME ?= $(PUBLIC_DOCKER_REGISTRY)/server:$(GIT_TAG)
DOCKER_QA_IMAGE_NAME ?= $(PUBLIC_DOCKER_REGISTRY)/server-qa:$(QA_DEV_TAG)
DOCKER_DEV_IMAGE_NAME ?= $(PUBLIC_DOCKER_REGISTRY)/server-dev:$(QA_DEV_TAG)


.PHONY: docker_build
docker_build: ## Build container for daemon
	docker build -t $(DOCKER_SERVER_IMAGE_NAME) .

.PHONY: docker_push
docker_push: docker ## Push container for daemon
	docker push $(DOCKER_SERVER_IMAGE_NAME)

.PHONY: qa_docker_build
qa_docker_build: ## Build container with QA test dependencies included
	docker build -t $(DOCKER_QA_IMAGE_NAME) -f ./Dockerfile.qa .

.PHONY: qa_docker_push
qa_docker_push: qa_docker_build ## Push container for daemon QA test environment
	docker push $(DOCKER_QA_IMAGE_NAME)

.PHONY: qa
qa:
	go build -o ./openbazaar-qa ./openbazaard.go
	(cd qa && ./runtests.sh ../openbazaar-qa /opt/bitcoin-0.16.3/bin/bitcoind)
	rm ./openbazaar-qa

.PHONY: qa_eth
qa_eth:
	go build -o ./openbazaar-qa ./openbazaard.go
	(cd qa && ./runtests_eth.sh ../openbazaar-qa)
	rm ./openbazaar-qa


.PHONY: dev_docker_build
dev_docker: ## Build container with dev dependencies included
	docker build -t $(DOCKER_DEV_IMAGE_NAME) -f ./Dockerfile.dev .

.PHONY: dev_docker_push
dev_docker_push: dev_docker_build ## Push container for daemon dev environment
	docker push $(DOCKER_DEV_IMAGE_NAME)
