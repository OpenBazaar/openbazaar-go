##
## Building
##

build: build_linux build_docker_image
build_linux:
	docker pull karalabe/xgo-latest
	go get github.com/karalabe/xgo
	mkdir dist; true
	(cd dist && xgo --targets=linux/amd64 ../)
	(chmod +x ./dist/*)

##
## docker
##
DOCKER_PROFILE ?= ob1company
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/openbazaard_dummy

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
