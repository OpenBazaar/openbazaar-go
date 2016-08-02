WINDOWS INSTALL NOTES
====================

### Install dependencies

You need to have Go, Git, and GCC installed to compile and run the OpenBazaar-Go daemon.

- **Go**
    + https://golang.org
- **Git**
    + https://git-scm.com/download/win
- **TDM-GCC/MinGW-w64**
    + http://tdm-gcc.tdragon.net/ 
    + A standard installation should automatically add the correct `PATH`, but if it doesn't:
        * Open the command prompt and type: `setx PATH "%PATH;C:\TDM-GCC-64\bin"`

### Setup Go

Create a directory to store all your Go projects (e.g. `C:\goprojects`):

- Set your GOPATH
    + Open the command prompt and type: `setx GOPATH "C:\goprojects"`

### Install openbazaar-go

Press the Windows button and type 'run', and then `MinGW Command Prompt' until you see the option appear. Click to open the 'MinGW Command Prompt'.

- Install the `openbazaar-go`:
    + Type: `go get github.com/OpenBazaar/openbazaar-go`
    + It will put the source code in `%GOPATH%\src\github.com\OpenBazaar\openbazaar-go`
- To compile and run `openbazaar-go`:
    + Navigate the source directory: `cd %GOPATH%\src\github.com\OpenBazaar\openbazaar-go` 
    + Type: `go run openbazaard.go start`
