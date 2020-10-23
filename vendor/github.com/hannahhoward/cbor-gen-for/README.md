# cbor-gen-for

Automatically add CBOR serialization/deserialization with go generate and cbor-gen

## Usage

- install with go mod

- in a file with types you want to generate cbor for, add a go-generate comment:

`objects.go`

```golang
package objects

//go:generate cbor-gen-for Car House

type Car struct {
  WheelType string
  HorsePower uint64
}

type House struct {
  Stories uint64
  Color string
}
```

- run `go generate ./...` to make cbor serialization code for your whole project

This will make cbor serialization files of the name *original-file-name*_cbor_gen.go in the original location

Note that the project must be compilable first to do this, so if you get errors, make sure `go build ./...` works first.

# Caveat

Since this is a generator that doesn't go into the actual code for your project, to retain during `go mod tidy` you should add a file at the root with a seperate build target, a.l.a. [https://github.com/go-modules-by-example/index/tree/master/010_tools](https://github.com/go-modules-by-example/index/tree/master/010_tools)

```
// +build tools

package tools

import (
	_ "github.com/hannahhoward/cbor-gen-for"
)
```

## License
This repository is dual-licensed under Apache 2.0 and MIT terms.

Copyright 2019. Protocol Labs, Inc.
