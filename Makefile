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
## docker
##
DOCKER_PROFILE ?= openbazaar
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/openbazaard

build_docker_image:
	docker build -t $(DOCKER_IMAGE_NAME) .

push_docker_image:
	docker push $(DOCKER_IMAGE_NAME)

docker: build_linux build_docker_image push_docker_image

##
## Cleanup
##

clean_build:
	rm -f ./dist/*

clean_docker:
	docker rmi -f $(DOCKER_IMAGE_NAME); true

clean: clean_build clean_docker
