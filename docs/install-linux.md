LINUX INSTALL NOTES
====================

### Install dependencies

You need to have Go (and git) installed to compile and run the daemon.
```
sudo add-apt-repository ppa:ubuntu-lxc/lxd-stable
sudo apt-get update
sudo apt-get install golang git
```

### Setup Go

Create a directory to store all your Go projects (below we just put the directory in our home directory but you can use any directory you want).

```
mkdir go
```

Set that directory as your go path:

Edit `.profile` in your home directory and append the following to the end of the file (if you used a different go directory make sure to change it below):
```
export GOPATH=$HOME/go
```

Then run:
```
source ~/.profile
```

Go should now be installed.

### Install openbazaar-go

```
go get github.com/OpenBazaar/openbazaar-go
```

It will put the source code in $GOPATH/src/github.com/OpenBazaar/openbazaar-go

To compile and run the source:
```
cd $GOPATH/src/github.com/OpenBazaar/openbazaar-go
go run openbazaard.go start
```