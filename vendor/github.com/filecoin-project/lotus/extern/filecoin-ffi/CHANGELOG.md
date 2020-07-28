# filecoin-ffi changelog

## 0.30.1

This release wil include Window PoSt speedups (2x measured for 32GiB sectors),
RAM reduction of 56GiB for 32GiB sectors (mmap'd parent cache with windows for
access, rather than all in RAM at once), and some phase2/trusted setup related
updates (for the trusted setup participants).

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - update to rust-fil-proofs 4.0.2 (#113) ([filecoin-project/filecoin-ffi#113](https://github.com/filecoin-project/filecoin-ffi/pull/113))
  - run the Rust tests before running the Go tests (#112) ([filecoin-project/filecoin-ffi#112](https://github.com/filecoin-project/filecoin-ffi/pull/112))
  - Update master dependencies ([filecoin-project/filecoin-ffi#111](https://github.com/filecoin-project/filecoin-ffi/pull/111))
  - update changelog for 0.30.0 release ([filecoin-project/filecoin-ffi#109](https://github.com/filecoin-project/filecoin-ffi/pull/109))
- github.com/filecoin-project/specs-actors (v0.6.0 -> v0.6.1)

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| nemo | 3 | +1463/-1782 | 17 |
| Alex North | 2 | +139/-116 | 3 |
| Erin Swenson-Healey | 2 | +90/-68 | 7 |
| laser | 1 | +31/-0 | 1 |

## 0.30.0

This release includes an update specs-actors (splits abi.RegisteredProof into
two new types - one for seal and one for PoSt) and an update to rust-fil-proofs
4.0.0, which you can read about [here](https://github.com/filecoin-project/rust-fil-proofs/blob/master/CHANGELOG.md#400---2020-06-15).

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - update to rust-fil-proofs 4.0.0 (#108) ([filecoin-project/filecoin-ffi#108](https://github.com/filecoin-project/filecoin-ffi/pull/108))
  - specs-actors v0.6 ([filecoin-project/filecoin-ffi#107](https://github.com/filecoin-project/filecoin-ffi/pull/107))
  - changelog for 0.28.0, 0.28.1, and 0.29.0 (#106) ([filecoin-project/filecoin-ffi#106](https://github.com/filecoin-project/filecoin-ffi/pull/106))
- github.com/filecoin-project/specs-actors (v0.5.4-0.20200521014528-0df536f7e461 -> v0.6.0)

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Frrist | 3 | +454/-239 | 22 |
| Whyrusleeping | 2 | +485/-119 | 19 |
| Alex North | 7 | +424/-151 | 15 |
| Łukasz Magiera | 4 | +227/-154 | 13 |
| Alex Cruikshank | 4 | +262/-85 | 10 |
| porcuquine | 1 | +172/-171 | 15 |
| Erin Swenson-Healey | 3 | +153/-30 | 5 |
| ZenGround0 | 3 | +42/-17 | 16 |
| ZX | 2 | +16/-19 | 4 |
| WEI YANG | 1 | +6/-2 | 1 |
| Henri | 1 | +2/-2 | 1 |


## 0.29.0

Big changes here! We moved off of the nightly Rust channel, fixed a nasty file
descriptor-leak, and (most importantly) updated to [v27 parameters and keys](https://github.com/filecoin-project/rust-fil-proofs/blob/master/CHANGELOG.md#300---2020-06-08).

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - fix: update to filecoin-proofs-api v3.0.0 ([filecoin-project/filecoin-ffi#105](https://github.com/filecoin-project/filecoin-ffi/pull/105))
  - explicitly close os.File to force release of file descriptor (#97) ([filecoin-project/filecoin-ffi#97](https://github.com/filecoin-project/filecoin-ffi/pull/97))
  - fix: use stable 1.43.1 release ([filecoin-project/filecoin-ffi#102](https://github.com/filecoin-project/filecoin-ffi/pull/102))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| nemo | 2 | +133/-132 | 4 |
| Erin Swenson-Healey | 1 | +47/-1 | 2 |
| Volker Mische | 1 | +1/-3 | 3 |


## 0.28.1

This release modifies the rust-filecoin-proofs-api dependency, downloading it
from crates.io instead of GitHub. No behavior changes.

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - fix: point to published filecoin-proofs-api crate ([filecoin-project/filecoin-ffi#104](https://github.com/filecoin-project/filecoin-ffi/pull/104))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| nemo | 1 | +6/-5 | 3 |


## 0.28.0

This release adds unseal-to-a-file-descriptor functionality to the API, improves
merkle tree cache usage, [and more](https://github.com/filecoin-project/rust-fil-proofs/blob/master/CHANGELOG.md#200---2020-05-27).

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - integrate rust-fil-proofs 2.0.0 release (#98) ([filecoin-project/filecoin-ffi#98](https://github.com/filecoin-project/filecoin-ffi/pull/98))
  - release notes for 0.27.0 (#96) ([filecoin-project/filecoin-ffi#96](https://github.com/filecoin-project/filecoin-ffi/pull/96))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Erin Swenson-Healey | 2 | +245/-371 | 9 |


## 0.27.0

This release migrates from specs-actors 0.4.1 to 0.5.4.

### Breaking Changes

- The `VerifySeal` function has been modified to accept the revamped `abi.SealVerifyInfo` type.

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - consume new abi.SealVerifyInfo structure (#89) ([filecoin-project/filecoin-ffi#89](https://github.com/filecoin-project/filecoin-ffi/pull/89))
  - add changelog and changelog generator (#95) ([filecoin-project/filecoin-ffi#95](https://github.com/filecoin-project/filecoin-ffi/pull/95))
- github.com/filecoin-project/go-bitfield (v0.0.0-20200416002808-b3ee67ec9060 -> v0.0.1):
  - zero out bitfields during subtraction when they end up empty ([filecoin-project/go-bitfield#4](https://github.com/filecoin-project/go-bitfield/pull/4))
  - Create go.yml
- github.com/filecoin-project/specs-actors (v0.4.1-0.20200509020627-3c96f54f3d7d -> v0.5.4-0.20200521014528-0df536f7e461):
  - decouple SealVerifyInfo from OnChainSealVerifyInfo (and rename to SealVerifyParams) (#378) ([filecoin-project/specs-actors#378](https://github.com/filecoin-project/specs-actors/pull/378))
  - add Unencodable Return method to puppet actor (#384) ([filecoin-project/specs-actors#384](https://github.com/filecoin-project/specs-actors/pull/384))
  - call validate caller (#379) ([filecoin-project/specs-actors#379](https://github.com/filecoin-project/specs-actors/pull/379))
  - handle last cron tick in market actor properly (#376) ([filecoin-project/specs-actors#376](https://github.com/filecoin-project/specs-actors/pull/376))
  - stop puppet actor panic when sendreturn is nil (#375) ([filecoin-project/specs-actors#375](https://github.com/filecoin-project/specs-actors/pull/375))
  - Change window post deadline duration to 1hr. (#373) ([filecoin-project/specs-actors#373](https://github.com/filecoin-project/specs-actors/pull/373))
  - cbor-gen for reward actor state (#372) ([filecoin-project/specs-actors#372](https://github.com/filecoin-project/specs-actors/pull/372))
  - Fractional network time (#367) ([filecoin-project/specs-actors#367](https://github.com/filecoin-project/specs-actors/pull/367))
  - deps: go-bitfield v0.0.1 (#369) ([filecoin-project/specs-actors#369](https://github.com/filecoin-project/specs-actors/pull/369))
  - update block reward target from KPI event, use that value to pay block rewards (#366) ([filecoin-project/specs-actors#366](https://github.com/filecoin-project/specs-actors/pull/366))
  - Helpers and mocks for miner window post unit tests. (#354) ([filecoin-project/specs-actors#354](https://github.com/filecoin-project/specs-actors/pull/354))
  - Change min miner power from 10TiB to 1TiB for testnet-2 (#368) ([filecoin-project/specs-actors#368](https://github.com/filecoin-project/specs-actors/pull/368))
  - Remove incorrect assumption about window post proof slice (#361) ([filecoin-project/specs-actors#361](https://github.com/filecoin-project/specs-actors/pull/361))
  - miner: Restrict supported proof types in miner ctor (#363) ([filecoin-project/specs-actors#363](https://github.com/filecoin-project/specs-actors/pull/363))
  - dont include value in the error message for set errors (#365) ([filecoin-project/specs-actors#365](https://github.com/filecoin-project/specs-actors/pull/365))
  - Fix nil verified deal weight (#360) ([filecoin-project/specs-actors#360](https://github.com/filecoin-project/specs-actors/pull/360))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Erin Swenson-Healey | 3 | +501/-293 | 12 |
| Łukasz Magiera | 4 | +388/-181 | 21 |
| Alex North | 5 | +346/-71 | 9 |
| Jakub Sztandera | 3 | +94/-9 | 5 |
| Whyrusleeping | 4 | +66/-36 | 9 |
| ZX | 1 | +21/-42 | 3 |
| Jeromy | 1 | +62/-0 | 2 |
| Frrist | 2 | +27/-8 | 2 |
| cerasusland | 1 | +2/-5 | 1 |

## 0.26.2

This release contains a fix for a bug which prevented unmodified miners from
generating Winning PoSts for 64GiB sectors. It also contains a fix for a bug
which was an occasional source of `bad file descriptor` errors observed during
CommP generation (our hypothesis is that the `*os.File` was being GC'ed before
the CGO call returned).

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - don't let Go garbage collect FD until FFI call returns ([filecoin-project/filecoin-ffi#84](https://github.com/filecoin-project/filecoin-ffi/pull/84))
  - fix: error if there is already a logger
  - add winning PoSt for 64 GiB (#93) ([filecoin-project/filecoin-ffi#93](https://github.com/filecoin-project/filecoin-ffi/pull/93))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Volker Mische | 1 | +24/-7 | 1 |
| laser | 1 | +5/-5 | 1 |
| shannon-6block | 1 | +2/-0 | 1 |

## 0.26.1

This release updates to version 0.4.1 of specs-actors, which (among other
things) extends the `RegisteredProof` types to include 64GiB sector sizes. It
also includes a fix for Window PoSt (multiple proofs were being flattened into
a single byte array) and various fixes for bellperson and neptune Rust crates.

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - Update deps revisited ([filecoin-project/filecoin-ffi#91](https://github.com/filecoin-project/filecoin-ffi/pull/91))
  - newest upstream (#88) ([filecoin-project/filecoin-ffi#88](https://github.com/filecoin-project/filecoin-ffi/pull/88))
  - update rust-filecoin-proofs-api to include PoSt fix (#87) ([filecoin-project/filecoin-ffi#87](https://github.com/filecoin-project/filecoin-ffi/pull/87))
  - upgrade to specs-actors 0.4.1 (64GiB sector support) ([filecoin-project/filecoin-ffi#85](https://github.com/filecoin-project/filecoin-ffi/pull/85))
  - Upgrade to specs-actors v0.3.0 (#81) ([filecoin-project/filecoin-ffi#81](https://github.com/filecoin-project/filecoin-ffi/pull/81))
- github.com/filecoin-project/go-amt-ipld (v2.0.1-0.20200131012142-05d80eeccc5e -> v2.0.1-0.20200424220931-6263827e49f2):
  - implement method to get first index in amt ([filecoin-project/go-amt-ipld#11](https://github.com/filecoin-project/go-amt-ipld/pull/11))
  - implement ForEachAt method to support iteration starting at a given i… ([filecoin-project/go-amt-ipld#10](https://github.com/filecoin-project/go-amt-ipld/pull/10))
- github.com/filecoin-project/specs-actors (v0.2.0 -> v0.4.1-0.20200509020627-3c96f54f3d7d):
  - Minting function maintainability (#356) ([filecoin-project/specs-actors#356](https://github.com/filecoin-project/specs-actors/pull/356))
  - support for 64GiB sectors (#355) ([filecoin-project/specs-actors#355](https://github.com/filecoin-project/specs-actors/pull/355))
  - Temporary param update (#352) ([filecoin-project/specs-actors#352](https://github.com/filecoin-project/specs-actors/pull/352))
  - document reward minting function tests (#348) ([filecoin-project/specs-actors#348](https://github.com/filecoin-project/specs-actors/pull/348))
  - puppet type and method for failed marshal to cbor (#347) ([filecoin-project/specs-actors#347](https://github.com/filecoin-project/specs-actors/pull/347))
  - Unit tests for prove commit sector (#351) ([filecoin-project/specs-actors#351](https://github.com/filecoin-project/specs-actors/pull/351))
  - Fix failure to detect faults of exactly-full top partition (#350) ([filecoin-project/specs-actors#350](https://github.com/filecoin-project/specs-actors/pull/350))
  - Fix checking of fault/recovery declaration deadlines (#349) ([filecoin-project/specs-actors#349](https://github.com/filecoin-project/specs-actors/pull/349))
  - Set ConsensusMinerMinPower to 10TiB (#344) ([filecoin-project/specs-actors#344](https://github.com/filecoin-project/specs-actors/pull/344))
  - improve deal accounting performance (#309) ([filecoin-project/specs-actors#309](https://github.com/filecoin-project/specs-actors/pull/309))
  - DeadlineInfo handles expired proving period (#343) ([filecoin-project/specs-actors#343](https://github.com/filecoin-project/specs-actors/pull/343))
  - document reward-minting taylorSeriesExpansion (#338) ([filecoin-project/specs-actors#338](https://github.com/filecoin-project/specs-actors/pull/338))
  - implement puppet actor (#290) ([filecoin-project/specs-actors#290](https://github.com/filecoin-project/specs-actors/pull/290))
  - Fix the 32GiB Window PoSt partition size again (#337) ([filecoin-project/specs-actors#337](https://github.com/filecoin-project/specs-actors/pull/337))
  - Fix seal proof type in miner actor and parameterize WPoSt partition size by it (#336) ([filecoin-project/specs-actors#336](https://github.com/filecoin-project/specs-actors/pull/336))
  - Change WPoStPartitionSectors to 2349 (#332) ([filecoin-project/specs-actors#332](https://github.com/filecoin-project/specs-actors/pull/332))
  - Remove unused SectorSize from VerifyDealsOnSectorProveCommitParams (#328) ([filecoin-project/specs-actors#328](https://github.com/filecoin-project/specs-actors/pull/328))
  - require success in reward actor send reward (#331) ([filecoin-project/specs-actors#331](https://github.com/filecoin-project/specs-actors/pull/331))
  - Power actor CreateMiner passes on value received to new actor (#327) ([filecoin-project/specs-actors#327](https://github.com/filecoin-project/specs-actors/pull/327))
  - Specify cron genesis entries (#326) ([filecoin-project/specs-actors#326](https://github.com/filecoin-project/specs-actors/pull/326))
  - Remove SysErrInternal definition, use of which is always a bug (#304) ([filecoin-project/specs-actors#304](https://github.com/filecoin-project/specs-actors/pull/304))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Alex North | 13 | +654/-280 | 35 |
| Whyrusleeping | 2 | +273/-437 | 13 |
| Frrist | 3 | +455/-6 | 7 |
| davidad (David A. Dalrymple) | 3 | +245/-46 | 5 |
| Jeromy | 2 | +166/-4 | 4 |
| laser | 4 | +110/-48 | 6 |
| Erin Swenson-Healey | 3 | +50/-30 | 5 |
| ZX | 1 | +48/-20 | 5 |
| nemo | 1 | +4/-56 | 2 |

## 0.26.0

This release migrates from v25 to v26 Groth parameters, which allows us to use
64GiB sectors. It also adds some safety to the CGO bindings, which were
previously sharing Go memory with C, resulting in some errors when running with
`cgocheck=2`.

### Changelog

- github.com/filecoin-project/filecoin-ffi:
  - update to v26 Groth parameters (#83) ([filecoin-project/filecoin-ffi#83](https://github.com/filecoin-project/filecoin-ffi/pull/83))
  - handle allocations for problematic structs to avoid sharing pointers-to-pointers with C (from Go) (#82) ([filecoin-project/filecoin-ffi#82](https://github.com/filecoin-project/filecoin-ffi/pull/82))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Erin Swenson-Healey | 2 | +514/-375 | 15 |
