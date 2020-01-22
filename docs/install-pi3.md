Pi 3, running [Raspbian Stretch Lite 4.9 kernel](https://www.raspberrypi.org/downloads/raspbian/) INSTALL NOTES

====================

### Install Git

You need to have gcc and git installed to compile and run the daemon.
```
sudo apt-get update
sudo apt-get install build-essential git -y
```

### Install Go

These are some condensed steps which will get you started quickly, but we recommend following the installation steps at [https://golang.org/doc/install](https://golang.org/doc/install).

Download Go 1.11 and extract executeables:
```
wget https://storage.googleapis.com/golang/go1.11.5.linux-armv6l.tar.gz
sudo tar -zxvf go1.11.5.linux-armv6l.tar.gz -C /usr/local/
```

Note: OpenBazaar has not been tested on v1.11 and may cause problems

### Setup Go

Create a directory to store all your Go projects (below we just put the directory in our home directory but you can use any directory you want).

```
mkdir $HOME/go
```

Set that directory as your go path:

Paste these lines at the command line, to append their quoted text to the end of `.profile` in your home directory (if you used a different go directory, make sure to change it below):

```
echo "export GOPATH=$HOME/go" >> .profile
echo "export PATH=$PATH:/usr/local/go/bin" >> .profile
```

Then run the command:
```
source ~/.profile
```

Go should now be installed.

### Install openbazaar-go

Checkout a copy of the source:
```
go get github.com/OpenBazaar/openbazaar-go
```

It will use git to checkout the source code into `$GOPATH/src/github.com/OpenBazaar/openbazaar-go`

Checkout a release version:
```
git checkout v0.13.8
```

Note: `go get` leaves the repo pointing at `master` which is a branch used for active development. Running OpenBazaar from `master` is NOT recommended. Check the [release versions](https://github.com/OpenBazaar/openbazaar-go/releases) for the available versions that you use iin checkout.

To compile and run the source using the path above, WITHOUT encrypting the database:
```
go run $GOPATH/src/github.com/OpenBazaar/openbazaar-go/openbazaard.go start
```
