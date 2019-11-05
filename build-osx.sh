#!/usr/bin/env bash

HOMEBREW_NO_AUTO_UPDATE=1
brew update-reset
brew install docker docker-compose docker-machine xhyve docker-machine-driver-xhyve
sudo chown root:wheel $(brew --prefix)/opt/docker-machine-driver-xhyve/bin/docker-machine-driver-xhyve
sudo chmod u+s $(brew --prefix)/opt/docker-machine-driver-xhyve/bin/docker-machine-driver-xhyve

brew services start docker-machine
sudo spctl --master-disable
brew cask install virtualbox
docker-machine create default --virtualbox-no-vtx-check

