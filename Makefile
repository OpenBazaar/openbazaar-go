##
## Building
##
deploy:
	./deploy.sh

build:
	./build.sh

linux_binary:
	./build.sh linux/amd64

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
DOCKER_SERVER_VERSION ?= $(shell git describe --tags --abbrev=0)
DOCKER_SERVER_IMAGE_NAME ?= $(DOCKER_PROFILE)/server:$(DOCKER_SERVER_VERSION)
DOCKER_DUMMY_IMAGE_NAME ?= $(DOCKER_PROFILE)/server_dummy:$(DOCKER_SERVER_VERSION)

docker:
	docker build -t $(DOCKER_SERVER_IMAGE_NAME) .

push_docker:
	docker push $(DOCKER_SERVER_IMAGE_NAME)

deploy_docker: docker push_docker

dummy_docker:
	docker build -t $(DOCKER_DUMMY_IMAGE_NAME) -f Dockerfile.dummy .

push_dummy_docker:
	docker push $(DOCKER_DUMMY_IMAGE_NAME)

deploy_dummy_docker: dummy_docker push_dummy_docker

##
## Cleanup
##
clean_build:
	rm -f ./dist/*

clean_docker:
	docker rmi -f $(DOCKER_SERVER_IMAGE_NAME) $(DOCKER_DUMMY_IMAGE_NAME) || true

clean: clean_build clean_docker
