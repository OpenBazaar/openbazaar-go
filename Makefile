##
## Building
##
deploy:
	./deploy.sh

build:
	./build.sh

build_linux:
	./build.sh linux/amd64



##
## Protobuf compilation
##
P_TIMESTAMP = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp
P_ANY = Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

PKGMAP = $(P_TIMESTAMP),$(P_ANY)

protos:
	cd pb/protos && protoc --go_out=$(PKGMAP):.. *.proto



##
## docker
##
DOCKER_PROFILE ?= openbazaar
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/openbazaard

build_docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

push_docker:
	docker push $(DOCKER_IMAGE_NAME)

docker: build_linux build_docker push_docker



##
## Cleanup
##
clean_build:
	rm -f ./dist/*

clean_docker:
	docker rmi -f $(DOCKER_IMAGE_NAME); true

clean: clean_build clean_docker
