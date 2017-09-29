LINUX INSTALL NOTES
====================

### Install dependencies

You need to have gcc and git installed to compile and run the daemon.
```
sudo apt-get update
sudo apt-get install build-essential git -y
```

### Install Go 1.8 or greater
```
wget https://storage.googleapis.com/golang/go1.7.1.linux-amd64.tar.gz
sudo tar -zxvf  go1.7.1.linux-amd64.tar.gz -C /usr/local/
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
export PATH=$PATH:/usr/local/go/bin
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
