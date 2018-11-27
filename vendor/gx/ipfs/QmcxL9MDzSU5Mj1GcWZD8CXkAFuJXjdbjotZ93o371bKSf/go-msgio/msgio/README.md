# msgio headers tool

Conveniently output msgio headers.

## Install

```
go get github.com/libp2p/go-msgio/msgio
```

## Usage

```
> msgio -h
msgio - tool to wrap messages with msgio header

Usage
    msgio header 1020 >header
    cat file | msgio wrap >wrapped

Commands
    header <size>   output a msgio header of given size
    wrap            wrap incoming stream with msgio
```
