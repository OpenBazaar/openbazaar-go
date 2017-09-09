Pi 3, running [Raspbian Stretch Lite 4.9 kernel](https://www.raspberrypi.org/downloads/raspbian/) INSTALL NOTES

====================

### Install dependencies

You need to have gcc and git installed to compile and run the daemon.
```
sudo apt-get update
sudo apt-get install build-essential git -y
```

### Install Go 1.9
```
wget https://storage.googleapis.com/golang/go1.9.linux-armv6l.tar.gz
sudo tar -zxvf go1.9.linux-armv6l.tar.gz -C /usr/local/
```

### Setup Go

Create a directory to store all your Go projects (below we just put the directory in our home directory but you can use any directory you want).
```
mkdir go
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
```
go get github.com/OpenBazaar/openbazaar-go
```

It will put the source code in 
```
$GOPATH/src/github.com/OpenBazaar/openbazaar-go
```

During the few minutes it takes the process to complete without a progress indicator, then return to blank command line, [read about securing your node](https://github.com/OpenBazaar/openbazaar-go/blob/master/docs/security.md)

To compile and run the source using the path above, WITHOUT encrypting the database:
```
go run $GOPATH/src/github.com/OpenBazaar/openbazaar-go/openbazaard.go start
```

You will then see 
```
________                      __________
\_____  \ ______   ____   ____\______   \_____  _____________  _____ _______
 /   |   \\____ \_/ __ \ /    \|    |  _/\__  \ \___   /\__  \ \__  \\_  __ \ 
/    |    \  |_> >  ___/|   |  \    |   \ / __ \_/    /  / __ \_/ __ \|  | \/
\_______  /   __/ \___  >___|  /______  /(____  /_____ \(____  (____  /__|
        \/|__|        \/     \/       \/      \/      \/     \/     \/
```
and the rest of the startup messages to follow.
