# Filecoin actors
[![CircleCI](https://circleci.com/gh/filecoin-project/specs-actors.svg?style=svg)](https://circleci.com/gh/filecoin-project/specs-actors)
[![codecov](https://codecov.io/gh/filecoin-project/specs-actors/branch/master/graph/badge.svg)](https://codecov.io/gh/filecoin-project/specs-actors)

This repo is the specification of the Filecoin builtin actors, in the form of executable code.

This is a companion to the rest of the [Filecoin Specification](https://github.com/filecoin-project/specs), 
but also directly usable by Go implementations of Filecoin.

## Versioning

Releases of this repo follow semantic versioning rules, with consideration of distributed state machines.
- The major version will remain `0` or `1` for the forseeable future. 
  We do not bump the major version every time there's a backwards-incompatible change in state machine evaluation, 
  or actor interfaces, because this interacts very poorly with Go's module resolution, 
  requiring a change of all import paths.
  After `1.0` we may consider using the major version number to version the `Runtime` interface, which is the link between
  the actors and the system in which they are embedded.
- A minor version change indicates a backwards-incompatible change in the state machine evaluation, including
  actor exported methods or constant values, while retaining compatibility of the `Runtime` interface.
  This means that the same sequence of messages might produce different states at two different versions.
  In a blockchain, this would usually require a coordinated network upgrade or "hard fork".
  After `1.0`, a minor version change may alter behaviour but not exported code or actor interfaces.
- A patch version change may alter state evaluation (but not exported code or actor interfaces).
  After `1.0`, a patch version change indicates a backward compatible fix or improvement that doesn't change
  state evaluation semantics or exported interfaces. 

## License
This repository is dual-licensed under Apache 2.0 and MIT terms.

Copyright 2019-2020. Protocol Labs, Inc.
