# go-data-transfer
[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![CircleCI](https://circleci.com/gh/filecoin-project/go-data-transfer.svg?style=svg)](https://circleci.com/gh/filecoin-project/go-data-transfer)
[![codecov](https://codecov.io/gh/filecoin-project/go-data-transfer/branch/master/graph/badge.svg)](https://codecov.io/gh/filecoin-project/go-data-transfer)

A go module to perform data transfers over [ipfs/go-graphsync](https://github.com/ipfs/go-graphsync)

## Description
This module encapsulates protocols for exchanging piece data between storage clients and miners, both when consummating a storage deal and when retrieving the piece later. 

## Table of Contents
* [Background](https://github.com/filecoin-project/go-data-transfer/tree/master#background)
* [Usage](https://github.com/filecoin-project/go-data-transfer/tree/master#usage)
    * [Initialize a data transfer module](https://github.com/filecoin-project/go-data-transfer/tree/master#initialize-a-data-transfer-module)
    * [Register a validator](https://github.com/filecoin-project/go-data-transfer/tree/master#register-a-validator)
    * [Open a Push or Pull Request](https://github.com/filecoin-project/go-data-transfer/tree/master#open-a-push-or-pull-request)
    * [Subscribe to Events](https://github.com/filecoin-project/go-data-transfer/tree/master#subscribe-to-events)
* [Contribute](https://github.com/filecoin-project/go-data-transfer/tree/master#contribute)

## Background

Please see the [design documentation](https://github.com/filecoin-project/go-data-transfer/tree/master/docs/DESIGNDOC)
for this module for a high-level overview and and explanation of the terms and concepts.

## Usage

**Requires go 1.13**

Install the module in your package or app with `go get "github.com/filecoin-project/go-data-transfer/datatransfer"`


### Initialize a data transfer module
1. Set up imports. You need, minimally, the following imports:
    ```go
    package mypackage

    import (
        gsimpl "github.com/ipfs/go-graphsync/impl"
        "github.com/filecoin-project/go-data-transfer/datatransfer"
        "github.com/libp2p/go-libp2p-core/host"
    )
            
    ```
1. Provide or create a [libp2p host.Host](https://github.com/libp2p/go-libp2p-examples/tree/master/libp2p-host)
1. Provide or create a [go-graphsync GraphExchange](https://github.com/ipfs/go-graphsync#initializing-a-graphsync-exchange)
1. Create a new instance of GraphsyncDataTransfer
    ```go
    func NewGraphsyncDatatransfer(h host.Host, gs graphsync.GraphExchange) {
        dt := datatransfer.NewGraphSyncDataTransfer(h, gs)
    }
    ```

1. If needed, build out your voucher struct and its validator. 
    
    A push or pull request must include a voucher. The voucher's type must have been registered with 
    the node receiving the request before it's sent, otherwise the request will be rejected.  

    [datatransfer.Voucher](https://github.com/filecoin-project/go-data-transfer/blob/21dd66ba370176224114b13030ee68cb785fadb2/datatransfer/types.go#L17)
    and [datatransfer.Validator](https://github.com/filecoin-project/go-data-transfer/blob/21dd66ba370176224114b13030ee68cb785fadb2/datatransfer/types.go#L153)
    are the interfaces used for validation of graphsync datatransfer messages.  Voucher types plus a Validator for them must be registered
    with the peer to whom requests will be sent.  

#### Example Toy Voucher and Validator
```go
type myVoucher struct {
	data string
}

func (v *myVoucher) ToBytes() ([]byte, error) {
	return []byte(v.data), nil
}

func (v *myVoucher) FromBytes(data []byte) error {
	v.data = string(data)
	return nil
}

func (v *myVoucher) Type() string {
	return "FakeDTType"
}

type myValidator struct {
	ctx                 context.Context
	validationsReceived chan receivedValidation
}

func (vl *myValidator) ValidatePush(
	sender peer.ID,
	voucher datatransfer.Voucher,
	baseCid cid.Cid,
	selector ipld.Node) error {
    
    v := voucher.(*myVoucher)
    if v.data == "" || v.data != "validpush" {
        return errors.New("invalid")
    }   

	return nil
}

func (vl *myValidator) ValidatePull(
	receiver peer.ID,
	voucher datatransfer.Voucher,
	baseCid cid.Cid,
	selector ipld.Node) error {

    v := voucher.(*myVoucher)
    if v.data == "" || v.data != "validpull" {
        return errors.New("invalid")
    }   

	return nil
}

```


Please see 
[go-data-transfer/blob/master/types.go](https://github.com/filecoin-project/go-data-transfer/blob/master/types.go) 
for more detail.


### Register a validator
Before sending push or pull requests, you must register a `datatransfer.Voucher` 
by its `reflect.Type` and `dataTransfer.RequestValidator` for vouchers that
must be sent with the request.  Using the trivial examples above:
```go
    func NewGraphsyncDatatransfer(h host.Host, gs graphsync.GraphExchange) {
        dt := datatransfer.NewGraphSyncDataTransfer(h, gs)

        vouch := &myVoucher{}
        mv := &myValidator{} 
        dt.RegisterVoucherType(reflect.TypeOf(vouch), mv)
    }
```
    
For more detail, please see the [unit tests](https://github.com/filecoin-project/go-data-transfer/blob/master/impl/graphsync/graphsync_impl_test.go).

### Open a Push or Pull Request
For a push or pull request, provide a context, a `datatransfer.Voucher`, a host recipient `peer.ID`, a baseCID `cid.CID` and a selector `ipld.Node`.  These
calls return a `datatransfer.ChannelID` and any error:
```go
    channelID, err := dtm.OpenPullDataChannel(ctx, recipient, voucher, baseCid, selector)
    // OR
    channelID, err := dtm.OpenPushDataChannel(ctx, recipient, voucher, baseCid, selector)

```

### Subscribe to Events

The module allows the consumer to be notified when a graphsync Request is sent or a datatransfer push or pull request response is received:

```go
    func ToySubscriberFunc (event Event, channelState ChannelState) {
        if event.Code == datatransfer.Error {
            // log error, flail about helplessly
            return
        }
        // 
        if channelState.Recipient() == our.PeerID && channelState.Received() > 0 {
            // log some stuff, update some state somewhere, send data to a channel, etc.
        }
    }

    dtm := SetupDataTransferManager(ctx, h, gs, baseCid, snode)
    unsubFunc := dtm.SubscribeToEvents(ToySubscriberFunc)

    // . . . later, when you don't need to know about events any more:
    unsubFunc()
```

## Contributing
PRs are welcome!  Please first read the design docs and look over the current code.  PRs against 
master require approval of at least two maintainers.  For the rest, please see our 
[CONTRIBUTING](https://github.com/filecoin-project/go-data-transfer/CONTRIBUTING.md) guide.

## License
This repository is dual-licensed under Apache 2.0 and MIT terms.

Copyright 2019. Protocol Labs, Inc.