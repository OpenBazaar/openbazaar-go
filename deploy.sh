#!/bin/bash

if ([ "$TRAVIS_BRANCH" == "master" ] && [ ! -z "$TRAVIS_TAG" ]) &&
    [ "$TRAVIS_PULL_REQUEST" == "false" ]; then
  echo "This will deploy!"
  export CGO_ENABLED=1
  docker pull karalabe/xgo-latest
  go get github.com/karalabe/xgo
  mkdir dist/ && cd dist/
  xgo --targets=windows/386,windows/amd64,darwin/amd64,linux/386,linux/amd64,linux/arm ../
  chmod +x *
  ghr --username OpenBazaar -t $GITHUB_TOKEN --replace --prerelease --debug $TRAVIS_TAG .
else
  echo "This will not deploy!"
fi
