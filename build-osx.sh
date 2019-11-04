#!/usr/bin/env bash

ulimit -n 1024

brew install docker docker-machine
brew cask install virtualbox
docker-machine create --driver virtualbox default
docker-machine env default
