MAC OS X INSTALL NOTES
====================

### Install Git

You need to have Go (and git) installed to compile and run the daemon.

```
brew install git
```

### Install Go
These are some condensed steps which will get you started quickly, but we recommend following the installation steps at [https://golang.org/doc/install](https://golang.org/doc/install).

Use brew to install Go 1.11:
```
brew install go@1.11
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
NOTE: If you have Xcode installed you may get the response `signal: killed`. If you do try running the following instead.

```
go run --ldflags -s openbazaard.go start -t
```

NOTE FOR NEW GOLANG HACKERS: 

In most projects you usually perform a `git clone` of the repository in order to start hacking. 

With `openbazaar-go` There's no need to manually `git clone` the project, this is done for you when you issue the `go get github.com/OpenBazaar/openbazaar-go` command, doing a manual `git clone` will only give you a repository that's missing a lot of recursive dependencies and building headaches.

If you are used to having all your other projects in some other place on disk, just make a symlink from `$GOPATH/src/github.com/OpenBazaar/openbazaar-go` into your usual workspace folder.

To start hacking make sure to add your git remote into the `$GOPATH/src/github.com/OpenBazaar/openbazaar-go` folder with:
```
cd $GOPATH/src/github.com/OpenBazaar/openbazaar-go
git remote add myusername git@github.com:myusername/openbazaar-go.git
```
