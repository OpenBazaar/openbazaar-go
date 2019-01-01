MAC OS X INSTALL NOTES
====================

### Install Git

You need to have Go (and git) installed to compile and run the daemon.

```
brew install git
```

### Install Go
These are some condensed steps which will get you started quickly, but we recommend following the installation steps at [https://golang.org/doc/install](https://golang.org/doc/install).

Use brew to install Go 1.10:
```
brew install go@1.10
```

Go may also be installed following the directions at [https://golang.org/doc/install](https://golang.org/doc/install).

Note: OpenBazaar has not been tested with v1.11 and may cause problems

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

Go should now be ready.

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
NOTE: If you have Xcode installed you may get the response `signal: killed`. If you do try running the following instead.

```
go run --ldflags -s openbazaard.go start -t
```
