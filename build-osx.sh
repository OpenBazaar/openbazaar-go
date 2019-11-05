#!/usr/bin/env bash

echo "Increase the maximum number of open file descriptors on macOS"
NOFILE=20480
sudo sysctl -w kern.maxfiles=$NOFILE
sudo sysctl -w kern.maxfilesperproc=$NOFILE
sudo launchctl limit maxfiles $NOFILE $NOFILE
sudo launchctl limit maxfiles
ulimit -S -n $NOFILE
ulimit -n

HOMEBREW_NO_AUTO_UPDATE=1
brew update-reset
brew install docker docker-compose docker-machine xhyve docker-machine-driver-xhyve
sudo chown root:wheel $(brew --prefix)/opt/docker-machine-driver-xhyve/bin/docker-machine-driver-xhyve
sudo chmod u+s $(brew --prefix)/opt/docker-machine-driver-xhyve/bin/docker-machine-driver-xhyve

