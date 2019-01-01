LINUX INSTALL NOTES
====================

### Install Git

You need to have gcc and git installed to compile and run the daemon.
```
sudo apt-get update
sudo apt-get install build-essential git -y
```

### Install Go
These are some condensed steps which will get you started quickly, but we recommend following the installation steps at [https://golang.org/doc/install](https://golang.org/doc/install).

Download Go 1.10 and extract executeables:
```
wget https://storage.googleapis.com/golang/go1.10.7.linux-amd64.tar.gz
sudo tar -zxvf  go1.10.7.linux-amd64.tar.gz -C /usr/local/
```

Note: OpenBazaar has not been tested on v1.11 and may cause problems

### Setup Go

Create a directory to store all your Go projects (below we just put the directory in our home directory but you can use any directory you want).

```
mkdir $HOME/go
```

Set that directory as your go path:

Edit `.profile` in your home directory and append the following to the end of the file (if you used a different go directory make sure to change it below):
```
echo "export GOPATH=$HOME/go" >> .profile
echo "export PATH=$PATH:/usr/local/go/bin" >> .profile
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

It will use git to checkout the source code into `$GOPATH/src/github.com/OpenBazaar/openbazaar-go`

To compile and run the source:
```
cd $GOPATH/src/github.com/OpenBazaar/openbazaar-go
go run openbazaard.go start
```
