#!/bin/bash

if [ ! -z "$TRAVIS_TAG" ] &&
    [ "$TRAVIS_PULL_REQUEST" == "false" ]; then
  echo "This will deploy!"

  # Cross-compile for all platforms
  export CGO_ENABLED=1
  docker pull karalabe/xgo-latest
  go get github.com/karalabe/xgo
  mkdir dist/ && cd dist/
  xgo --targets=windows/386,windows/amd64,darwin/amd64,linux/386,linux/amd64,linux/arm ../
  chmod +x *

  # Calculate SHA512 hashes
  sha512sum * > sha512_checksums.txt

  # Load signing key
  cp ../.travis/sign.key.gpg /tmp
  gpg --yes --batch --passphrase=$GPG_PASS /tmp/sign.key.gpg
  gpg --allow-secret-key-import --import /tmp/sign.key.gpg
  rm /tmp/sign.key.gpg

  # Sign hash file
  gpg --clearsign --digest-algo SHA256 --armor --output sha512_checksums.asc --passphrase=$GPG_PASS --default-key $GPG_KEYID sha512_checksums.txt

  rm sha512_checksums.txt

  # Upload to GitHub Release page
  ghr --username OpenBazaar -t $GITHUB_TOKEN --replace --prerelease --debug $TRAVIS_TAG .
else
  echo "This will not deploy!"
fi
