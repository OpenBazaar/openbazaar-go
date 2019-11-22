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

Download Go 1.11 and extract executeables:
```
wget https://storage.googleapis.com/golang/go1.11.5.linux-amd64.tar.gz
sudo tar -zxvf  go1.11.5.linux-amd64.tar.gz -C /usr/local/
```

Note: OpenBazaar has not been tested on v1.12 and may cause problems

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

Checkout a copy of the source:
```
go get github.com/OpenBazaar/openbazaar-go
```

It will use git to checkout the source code into `$GOPATH/src/github.com/OpenBazaar/openbazaar-go`

Checkout a release version:
```
git checkout v0.13.6
```

Note: `go get` leaves the repo pointing at `master` which is a branch used for active development. Running OpenBazaar from `master` is NOT recommended. Check the [release versions](https://github.com/OpenBazaar/openbazaar-go/releases) for the available versions that you use in checkout.
To compile and run the source:
```
cd $GOPATH/src/github.com/OpenBazaar/openbazaar-go
go run openbazaard.go start
```

NOTE FOR NEW GOLANG HACKERS: 

In most projects you usually perform a `git clone` of the repository in order to start hacking. 

With `openbazaar-go` There's no need to manually `git clone` the project, this is done for you when you issue the `go get github.com/OpenBazaar/openbazaar-go` command, doing a manual `git clone` will only give you a repository that's missing a lot of recursive dependencies and building headaches.

If you are used to having all your other projects in some other place on disk, just make a symlink from `$GOPATH/src/github.com/OpenBazaar/openbazaar-go` into your usual workspace folder.

To start hacking and committing to your fork make sure to add your git remote into the `$GOPATH/src/github.com/OpenBazaar/openbazaar-go` folder with:

```
cd $GOPATH/src/github.com/OpenBazaar/openbazaar-go
git remote add myusername git@github.com:myusername/openbazaar-go.git
```
