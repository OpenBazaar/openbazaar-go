VERSION=1.72
GOBIN=go
GOGET=$(GOBIN) get
GOINSTALL=$(GOBIN) install
PATH:=/usr/local/bin:$(GOPATH)/bin:$(PATH)
BINARY_NAME=tokenbalance
XGO=GOPATH=$(GOPATH) $(GOPATH)/bin/xgo -go 1.10.x --dest=build
BUILDVERSION=-ldflags="-X main.VERSION=$(VERSION)"

all: dep install clean

release: dep build-all compress

publish: docker docker-push clean

static:
	$(GOBIN) build -ldflags="-X main.VERSION=$VERSION -linkmode external -extldflags -static" -o tokenbalance -v ./cmd

build:
	$(GOBIN) build -ldflags="-X main.VERSION=$(VERSION)" -o $(BINARY_NAME) -v ./cmd

install: build
	mv $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	$(GOPATH)/bin/$(BINARY_NAME) version

run: build
	./$(BINARY_NAME)

test:
	$(GOBIN) test -p 1 -ldflags="-X main.VERSION=$(VERSION)" -coverprofile=coverage.out -v ./...

coverage:
	$(GOPATH)/bin/goveralls -coverprofile=coverage.out -service=travis -repotoken $(COVERALLS)

build-all: clean
	mkdir build
	$(XGO) $(BUILDVERSION) --targets=linux/amd64 ./cmd
	$(XGO) $(BUILDVERSION) --targets=linux/386 ./cmd
	$(XGO) $(BUILDVERSION) --targets=linux/arm-7 ./cmd
	$(XGO) $(BUILDVERSION) --targets=linux/arm64 ./cmd
	$(XGO) $(BUILDVERSION) --targets=darwin/amd64 ./cmd
	$(XGO) $(BUILDVERSION) --targets=darwin/386 ./cmd
	$(XGO) $(BUILDVERSION) --targets=windows-6.0/amd64 ./cmd
	$(XGO) $(BUILDVERSION) --targets=windows-6.0/386 ./cmd
	$(XGO) --targets=linux/amd64 -ldflags="-X main.VERSION=$VERSION -linkmode external -extldflags -static" -out alpine ./cmd

build-alpine:
	$(XGO) --targets=linux/amd64 -ldflags="-X main.VERSION=$VERSION -linkmode external -extldflags -static" -out alpine ./cmd

docker:
	docker build --no-cache -t hunterlong/tokenbalance:latest --build-arg VERSION=$(VERSION) .
	docker tag hunterlong/tokenbalance:latest hunterlong/tokenbalance:v$(VERSION)

# push the :latest tag to Docker hub
docker-push:
	docker push hunterlong/tokenbalance:latest
	docker push hunterlong/tokenbalance:v$(VERSION)

# run dep to install all required golang dependecies
dep:
	dep ensure -vendor-only

dev-deps:
	$(GOGET) github.com/stretchr/testify/assert
	$(GOGET) golang.org/x/tools/cmd/cover
	$(GOGET) github.com/mattn/goveralls
	$(GOINSTALL) github.com/mattn/goveralls
	$(GOGET) github.com/rendon/testcli
	$(GOGET) github.com/karalabe/xgo
	$(GOGET) github.com/ethereum/go-ethereum
	$(GOGET) -d ./...

clean:
	rm -f tokenbalance
	rm -rf vendor
	rm -rf build
	rm -f coverage.out

tag:
	git tag "v$(VERSION)" --force

compress:
	cd build && mv alpine-linux-amd64 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-linux-alpine.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd build && mv cmd-darwin-10.6-amd64 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-osx-x64.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd build && mv cmd-darwin-10.6-386 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-osx-x32.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd build && mv cmd-linux-amd64 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-linux-x64.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd build && mv cmd-linux-386 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-linux-x32.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd build && mv cmd-windows-6.0-amd64.exe $(BINARY_NAME).exe
	cd build && zip $(BINARY_NAME)-windows-x64.zip $(BINARY_NAME).exe  && rm -f $(BINARY_NAME).exe
	cd build && mv cmd-windows-6.0-386.exe $(BINARY_NAME).exe
	cd build && zip $(BINARY_NAME)-windows-x32.zip $(BINARY_NAME).exe  && rm -f $(BINARY_NAME).exe
	cd build && mv cmd-linux-arm-7 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-linux-arm7.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd build && mv cmd-linux-arm64 $(BINARY_NAME)
	cd build && tar -czvf $(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)

.PHONY: build test compress all publish release