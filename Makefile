##
## Building
##
deploy:
	./deploy.sh

build:
	./build.sh

linux_binary:
	./build.sh linux/amd64

ios_framework:
	gomobile bind -target=ios/arm64 github.com/OpenBazaar/openbazaar-go/mobile

android_framework:
	gomobile bind -target=android github.com/OpenBazaar/openbazaar-go/mobile

##
## Protobuf compilation
##
P_TIMESTAMP = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp
P_ANY = Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

PKGMAP = $(P_TIMESTAMP),$(P_ANY)

protos:
	cd pb/protos && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=$(PKGMAP):.. *.proto

##
## Docker
##
DOCKER_PROFILE ?= openbazaar
DOCKER_VERSION ?= $(shell git describe --tags --abbrev=0)
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/server:$(DOCKER_VERSION)

docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

push_docker:
	docker push $(DOCKER_IMAGE_NAME)

deploy_docker: docker push_docker

##
## Cleanup
##
clean_build:
	rm -f ./dist/*

clean_docker:
	docker rmi -f $(DOCKER_IMAGE_NAME) || true

clean: clean_build clean_docker
