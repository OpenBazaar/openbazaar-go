WINDOWS INSTALL NOTES
====================

### Install dependencies

You need to have Go, Git, and GCC installed to compile and run the daemon.

Download and install the following:

Go:  https://golang.org/dl/

Git: https://git-scm.com/download/win

GCC: http://tdm-gcc.tdragon.net/

### Setup Go

Create a directory to store all your Go projects: ex) C:\goprojects

Set your GOPATH

```
setx GOPATH "C:\goprojects"
```

Add the C compiler to your PATH
```
setx PATH "%PATH;C:\TDM-GCC-64\bin"
```

Go should now be installed.

### Install openbazaar-go

```
go get github.com/OpenBazaar/openbazaar-go
```

It will put the source code in %GOPATH%\src\github.com\OpenBazaar\openbazaar-go

To compile and run the source (this assumes you set C:\goprojects as your GOPATH, if not use your directory):
```
cd C:\goprojects\src\github.com\OpenBazaar\openbazaar-go
go run openbazaard.go start
```