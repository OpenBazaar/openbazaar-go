MAC OS X INSTALL NOTES
====================

### Install dependencies with [Homebrew](http://brew.sh/)

You need to have Go (and git) installed to compile and run the daemon.

```
brew install git
brew install go
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

Go should now be ready.

### Install openbazaar-go

```
go get github.com/OpenBazaar/openbazaar-go
```

It will put the source code in `$GOPATH/src/github.com/OpenBazaar/openbazaar-go`

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
