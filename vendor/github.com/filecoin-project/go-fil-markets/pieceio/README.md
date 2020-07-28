# pieceio

The pieceio module is a collection of structs for generating piece commitments (a.k.a. CommP) and 
storing pieces for storage market deals. It is used by the 
[`storagemarket`](../storagemarket) module.

## Installation
```bash
go get github.com/filecoin-project/go-fil-markets/pieceio
```

## PieceIO
`PieceIO` is used by [`storagemarket`](../storagemarket) client for proposing deals. 

**To initialize a PieceIO:**
```go
package pieceio

func NewPieceIO(carIO CarIO, bs blockstore.Blockstore) PieceIO
```
**Parameters**
* `carIO` is a [CarIO](#CarIO) from this module
* `bs` is an IPFS blockstore for storing and retrieving data for deals. See
 [github.com/ipfs/go-ipfs-blockstore](github.com/ipfs/go-ipfs-blockstore).

## PieceIOWithStore
`PieceIOWithStore` is `PieceIO` with a [`filestore`](../filestore). It is used by 
[`storagemarket`](../storagemarket) provider to store pieces, and to generate and store piece commitments
 and piece metadata for deals. 
 
**To initialize a PieceIOWithStore:**

```go
package pieceio

func NewPieceIOWithStore(carIO CarIO, store filestore.FileStore, bs blockstore.Blockstore) PieceIOWithStore
```
**Parameters**
* `carIO` is a [CarIO](#CarIO) from this module
* `store` is a [FileStore](../filestore) from this go-fil-markets repo.
* `bs` is an IPFS blockstore for storing and retrieving data for deals. See
 [github.com/ipfs/go-ipfs-blockstore](github.com/ipfs/go-ipfs-blockstore).

## CarIO
CarIO is a utility module that wraps [github.com/ipld/go-car](https://github.com/ipld/go-car) for use by storagemarket.

**To initialize a CarIO:**
```go
package cario

func NewCarIO() pieceio.CarIO
```

Please the [tests](pieceio_test.go) for more information about expected behavior.