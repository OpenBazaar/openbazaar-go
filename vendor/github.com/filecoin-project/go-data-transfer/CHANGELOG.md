# go-data-transfer changelog

# go-data-transfer 0.9.0

Major release of the 1.1 data transfer protocol, which supports restarts of data transfers.

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Message compatibility on graphsync (#102) ([filecoin-project/go-data-transfer#102](https://github.com/filecoin-project/go-data-transfer/pull/102))
  - Handle network errors/stalls (#101) ([filecoin-project/go-data-transfer#101](https://github.com/filecoin-project/go-data-transfer/pull/101))
  - Resume Data Transfer (#100) ([filecoin-project/go-data-transfer#100](https://github.com/filecoin-project/go-data-transfer/pull/100))
  - docs(CHANGELOG): update for 0.6.7 release ([filecoin-project/go-data-transfer#98](https://github.com/filecoin-project/go-data-transfer/pull/98))
- github.com/ipfs/go-graphsync (v0.2.1 -> v0.3.0):
  - feat(CHANGELOG): update for 0.3.0
  - docs(CHANGELOG): update for 0.2.1 ([ipfs/go-graphsync#103](https://github.com/ipfs/go-graphsync/pull/103))
  - Track actual network operations in a response (#102) ([ipfs/go-graphsync#102](https://github.com/ipfs/go-graphsync/pull/102))
  - feat(responsecache): prune blocks more intelligently (#101) ([ipfs/go-graphsync#101](https://github.com/ipfs/go-graphsync/pull/101))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Aarsh Shah | 1 | +9597/-2220 | 67 |
| Hannah Howard | 4 | +2355/-1018 | 51 |
| hannahhoward | 1 | +25/-3 | 4 |

# go-data-transfer 0.6.7

Minor update w/ fixes to support go-fil-markets 0.7.0

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Feat/cleanup errors (#90) ([filecoin-project/go-data-transfer#90](https://github.com/filecoin-project/go-data-transfer/pull/90))
  - Disambiguate whether a revalidator recognized a request when checking for a need to revalidate (#87) ([filecoin-project/go-data-transfer#87](https://github.com/filecoin-project/go-data-transfer/pull/87))
  - docs(CHANGELOG): update for 0.6.6 ([filecoin-project/go-data-transfer#89](https://github.com/filecoin-project/go-data-transfer/pull/89))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 2 | +167/-30 | 9 |

# go-data-transfer 0.6.6

Dependency update - go graphsync fix

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - feat(deps): update graphsync (#86) ([filecoin-project/go-data-transfer#86](https://github.com/filecoin-project/go-data-transfer/pull/86))
  - docs(CHANGELOG): updates for 0.6.5 ([filecoin-project/go-data-transfer#85](https://github.com/filecoin-project/go-data-transfer/pull/85))
- github.com/ipfs/go-graphsync (v0.2.0 -> v0.2.1):
  - docs(CHANGELOG): update for 0.2.1
  - Release/0.2.0 ([ipfs/go-graphsync#99](https://github.com/ipfs/go-graphsync/pull/99))
  - fix(metadata): fix cbor-gen (#98) ([ipfs/go-graphsync#98](https://github.com/ipfs/go-graphsync/pull/98))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| hannahhoward | 1 | +83/-68 | 1 |
| Hannah Howard | 2 | +15/-19 | 5 |

# go-data-transfer 0.6.5

Dependency update - go-graphsync and go-ipld-prime

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - feat(deps): update graphsync 0.2.0 (#83) ([filecoin-project/go-data-transfer#83](https://github.com/filecoin-project/go-data-transfer/pull/83))
  - docs(CHANGELOG): update for 0.6.4 ([filecoin-project/go-data-transfer#82](https://github.com/filecoin-project/go-data-transfer/pull/82))
- github.com/hannahhoward/cbor-gen-for (v0.0.0-20191218204337-9ab7b1bcc099 -> v0.0.0-20200817222906-ea96cece81f1):
  - add flag to select map encoding ([hannahhoward/cbor-gen-for#1](https://github.com/hannahhoward/cbor-gen-for/pull/1))
  - fix(deps): update cbor-gen-to-latest
- github.com/ipfs/go-graphsync (v0.1.2 -> v0.2.0):
  - docs(CHANGELOG): update for 0.2.0
  - style(imports): fix imports
  - fix(selectorvalidator): memory optimization (#97) ([ipfs/go-graphsync#97](https://github.com/ipfs/go-graphsync/pull/97))
  - Update go-ipld-prime@v0.5.0 (#92) ([ipfs/go-graphsync#92](https://github.com/ipfs/go-graphsync/pull/92))
  - refactor(metadata): use cbor-gen encoding (#96) ([ipfs/go-graphsync#96](https://github.com/ipfs/go-graphsync/pull/96))
  - Release/v0.1.2 ([ipfs/go-graphsync#95](https://github.com/ipfs/go-graphsync/pull/95))
  - Return Request context cancelled error (#93) ([ipfs/go-graphsync#93](https://github.com/ipfs/go-graphsync/pull/93))
  - feat(benchmarks): add p2p stress test (#91) ([ipfs/go-graphsync#91](https://github.com/ipfs/go-graphsync/pull/91))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Eric Myhre | 1 | +2919/-121 | 39 |
| Hannah Howard | 4 | +453/-143 | 29 |
| hannahhoward | 3 | +83/-63 | 10 |
| whyrusleeping | 1 | +31/-18 | 2 |
| Aarsh Shah | 1 | +27/-1 | 3 |

# go-data-transfer 0.6.4

security fix for messages

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Ensure valid messages are returned from FromNet() (#74) ([filecoin-project/go-data-transfer#74](https://github.com/filecoin-project/go-data-transfer/pull/74))
  - Release/v0.6.3 ([filecoin-project/go-data-transfer#70](https://github.com/filecoin-project/go-data-transfer/pull/70))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Ingar Shu | 1 | +20/-1 | 2 |

# go-data-transfer 0.6.3

Primarily a bug fix release-- graphsync performance and some better shutdown
logic

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - fix(deps): update graphsync, small cleanup
  - Stop data transfer correctly and some minor cleanp (#69) ([filecoin-project/go-data-transfer#69](https://github.com/filecoin-project/go-data-transfer/pull/69))
  - docs(CHANGELOG): update for 0.6.2 release ([filecoin-project/go-data-transfer#68](https://github.com/filecoin-project/go-data-transfer/pull/68))
- github.com/ipfs/go-graphsync (v0.1.1 -> v0.1.2):
  - fix(asyncloader): remove send on close channel
  - docs(CHANGELOG): update for 0.1.2 release
  - Benchmark framework + First memory fixes (#89) ([ipfs/go-graphsync#89](https://github.com/ipfs/go-graphsync/pull/89))
  - docs(CHANGELOG): update for v0.1.1 ([ipfs/go-graphsync#85](https://github.com/ipfs/go-graphsync/pull/85))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +1055/-39 | 17 |
| Aarsh Shah | 1 | +53/-68 | 8 |
| hannahhoward | 3 | +67/-34 | 11 |

# go-data-transfer 0.6.2

Minor bug fix release for request cancelling

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Fix Pull Request Cancelling (#67) ([filecoin-project/go-data-transfer#67](https://github.com/filecoin-project/go-data-transfer/pull/67))
  - docs(CHANGELOG): update for 0.6.1 ([filecoin-project/go-data-transfer#66](https://github.com/filecoin-project/go-data-transfer/pull/66))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +265/-9 | 4 |

# go-data-transfer 0.6.1

Update graphsync with critical bug fix for multiple transfers across custom stores

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Update graphsync 0.1.1 (#65) ([filecoin-project/go-data-transfer#65](https://github.com/filecoin-project/go-data-transfer/pull/65))
- github.com/ipfs/go-graphsync (v0.1.0 -> v0.1.1):
  - docs(CHANGELOG): update for v0.1.1
  - docs(CHANGELOG): update for v0.1.0 release ([ipfs/go-graphsync#84](https://github.com/ipfs/go-graphsync/pull/84))
  - Dedup by key extension (#83) ([ipfs/go-graphsync#83](https://github.com/ipfs/go-graphsync/pull/83))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 2 | +456/-57 | 17 |
| hannahhoward | 1 | +18/-1 | 2 |

# go-data-transfer 0.6.0

Includes two small breaking change updates:

- Update go-ipfs-blockstore to address blocks by-hash instead of by-cid. This brings go-data-transfer in-line with lotus.
- Update cbor-gen for some performance improvements and some API-breaking changes.

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Update cbor-gen (#63) ([filecoin-project/go-data-transfer#63](https://github.com/filecoin-project/go-data-transfer/pull/63))

### Contributors

| Contributor  | Commits | Lines ± | Files Changed |
|--------------|---------|---------|---------------|
| Steven Allen |       1 | +30/-23 |             5 |

# go-data-transfer 0.5.3

Minor fixes + update to release process

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - fix(deps): update graphsync
  - Release infrastructure (#61) ([filecoin-project/go-data-transfer#61](https://github.com/filecoin-project/go-data-transfer/pull/61))
  - Update cbor-gen (#60) ([filecoin-project/go-data-transfer#60](https://github.com/filecoin-project/go-data-transfer/pull/60))
- github.com/ipfs/go-graphsync (v0.0.6-0.20200731020347-9ff2ade94aa4 -> v0.1.0):
  - docs(CHANGELOG): update for v0.1.0 release
  - Release infrastructure (#81) ([ipfs/go-graphsync#81](https://github.com/ipfs/go-graphsync/pull/81))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 2 | +1202/-223 | 91 |
| Łukasz Magiera | 1 | +176/-176 | 8 |
| hannahhoward | 2 | +48/-3 | 3 |

# go-data-transfer 0.5.2

Security fix release

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - fix(deps): update graphsync
  - fix(message): add error check to FromNet (#59) ([filecoin-project/go-data-transfer#59](https://github.com/filecoin-project/go-data-transfer/pull/59))
- github.com/ipfs/go-graphsync (v0.0.6-0.20200721211002-c376cbe14c0a -> v0.0.6-0.20200731020347-9ff2ade94aa4):
  - feat(persistenceoptions): add unregister ability (#80) ([ipfs/go-graphsync#80](https://github.com/ipfs/go-graphsync/pull/80))
  - fix(message): regen protobuf code (#79) ([ipfs/go-graphsync#79](https://github.com/ipfs/go-graphsync/pull/79))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 3 | +442/-302 | 7 |
| hannahhoward | 1 | +3/-3 | 2 |

# go-data-transfer v0.5.1

Support custom configruation of transports

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Allow custom configuration of transports (#57) ([filecoin-project/go-data-transfer#57](https://github.com/filecoin-project/go-data-transfer/pull/57))
- github.com/ipfs/go-graphsync (v0.0.6-0.20200715204712-ef06b3d32e83 -> v0.0.6-0.20200721211002-c376cbe14c0a):
  - feat(persistenceoptions): add unregister ability

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +1049/-751 | 35 |
| hannahhoward | 1 | +113/-32 | 5 |

# go-data-transfer 0.5.0

Additional changes to support implementation of retrieval on top of go-data-transfer

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Minor fixes for retrieval on data transfer (#56) ([filecoin-project/go-data-transfer#56](https://github.com/filecoin-project/go-data-transfer/pull/56))
- github.com/ipfs/go-graphsync (v0.0.6-0.20200708073926-caa872f68b2c -> v0.0.6-0.20200715204712-ef06b3d32e83):
  - feat(requestmanager): run response hooks on completed requests (#77) ([ipfs/go-graphsync#77](https://github.com/ipfs/go-graphsync/pull/77))
  - Revert "add extensions on complete (#76)"
  - add extensions on complete (#76) ([ipfs/go-graphsync#76](https://github.com/ipfs/go-graphsync/pull/76))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 3 | +515/-218 | 26 |
| hannahhoward | 1 | +155/-270 | 9 |

# go-data-transfer 0.4.0

Major rewrite of library -- transports, persisted state, revalidators, etc. To support retrieval

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - The new data transfer (#55) ([filecoin-project/go-data-transfer#55](https://github.com/filecoin-project/go-data-transfer/pull/55))
  - Actually track progress for send/receive (#53) ([filecoin-project/go-data-transfer#53](https://github.com/filecoin-project/go-data-transfer/pull/53))
- github.com/ipfs/go-graphsync (v0.0.6-0.20200504202014-9d5f2c26a103 -> v0.0.6-0.20200708073926-caa872f68b2c):
  - All changes to date including pause requests & start paused, along with new adds for cleanups and checking of execution (#75) ([ipfs/go-graphsync#75](https://github.com/ipfs/go-graphsync/pull/75))
  - More fine grained response controls (#71) ([ipfs/go-graphsync#71](https://github.com/ipfs/go-graphsync/pull/71))
  - Refactor request execution and use IPLD SkipMe functionality for proper partial results on a request (#70) ([ipfs/go-graphsync#70](https://github.com/ipfs/go-graphsync/pull/70))
  - feat(graphsync): implement do-no-send-cids extension (#69) ([ipfs/go-graphsync#69](https://github.com/ipfs/go-graphsync/pull/69))
  - Incoming Block Hooks (#68) ([ipfs/go-graphsync#68](https://github.com/ipfs/go-graphsync/pull/68))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 7 | +12381/-4583 | 133 |

# go-data-transfer 0.3.0

Additional refactors to refactors to registry

### Changelog 
- github.com/filecoin-project/go-data-transfer:
  - feat(graphsyncimpl): fix open/close events (#52) ([filecoin-project/go-data-transfer#52](https://github.com/filecoin-project/go-data-transfer/pull/52))
  - chore(deps): update graphsync ([filecoin-project/go-data-transfer#51](https://github.com/filecoin-project/go-data-transfer/pull/51))
  - Refactor registry and encoding (#50) ([filecoin-project/go-data-transfer#50](https://github.com/filecoin-project/go-data-transfer/pull/50))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 2 | +993/-496 | 30 |

# go-data-transfer 0.2.1

Bug fix release -- critical nil check

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - chore(deps): update graphsync
- github.com/ipfs/go-graphsync (v0.0.6-0.20200428204348-97a8cf76a482 -> v0.0.6-0.20200504202014-9d5f2c26a103):
  - fix(responsemanager): add nil check (#67) ([ipfs/go-graphsync#67](https://github.com/ipfs/go-graphsync/pull/67))
  - Add autocomment configuration

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hector Sanjuan | 1 | +68/-0 | 1 |
| hannahhoward | 1 | +3/-3 | 2 |
| Hannah Howard | 1 | +4/-0 | 1 |

# go-data-transfer 0.2.0

Initial extracted release for Testnet Phase 2 (v0.1.0 + v0.1.1 were lotus tags prior to extraction)

### Changelog

- github.com/filecoin-project/go-data-transfer:
  - Upgrade graphsync + ipld-prime (#49) ([filecoin-project/go-data-transfer#49](https://github.com/filecoin-project/go-data-transfer/pull/49))
  - Use extracted generic pubsub (#48) ([filecoin-project/go-data-transfer#48](https://github.com/filecoin-project/go-data-transfer/pull/48))
  - Refactor & Cleanup In Preparation For Added Complexity (#47) ([filecoin-project/go-data-transfer#47](https://github.com/filecoin-project/go-data-transfer/pull/47))
  - feat(graphsync): complete notifications for responder (#46) ([filecoin-project/go-data-transfer#46](https://github.com/filecoin-project/go-data-transfer/pull/46))
  - Update graphsync ([filecoin-project/go-data-transfer#45](https://github.com/filecoin-project/go-data-transfer/pull/45))
  - docs(docs): remove outdated docs
  - docs(README): clean up README
  - docs(license): add license + contrib
  - ci(circle): add config
  - build(datatransfer): add go.mod
  - build(cbor-gen): add tools for cbor-gen-for .
  - fix links in datatransfer README (#11) ([filecoin-project/go-data-transfer#11](https://github.com/filecoin-project/go-data-transfer/pull/11))
  - feat(shared): add shared tools and types (#9) ([filecoin-project/go-data-transfer#9](https://github.com/filecoin-project/go-data-transfer/pull/9))
  - Feat/datatransfer readme, contributing, design doc (rename)
  - refactor(datatransfer): move to local module
  - feat(datatransfer): switch to graphsync implementation
  - Don't respond with error in gsReqRcdHook when we can't find the datatransfer extension. (#754) ([filecoin-project/go-data-transfer#754](https://github.com/filecoin-project/go-data-transfer/pull/754))
  - Feat/dt subscribe, file Xfer round trip (#720) ([filecoin-project/go-data-transfer#720](https://github.com/filecoin-project/go-data-transfer/pull/720))
  - Feat/dt gs pullrequests (#693) ([filecoin-project/go-data-transfer#693](https://github.com/filecoin-project/go-data-transfer/pull/693))
  - DTM sends data over graphsync for validated push requests (#665) ([filecoin-project/go-data-transfer#665](https://github.com/filecoin-project/go-data-transfer/pull/665))
  - Techdebt/dt split graphsync impl receiver (#651) ([filecoin-project/go-data-transfer#651](https://github.com/filecoin-project/go-data-transfer/pull/651))
  - Feat/dt initiator cleanup (#645) ([filecoin-project/go-data-transfer#645](https://github.com/filecoin-project/go-data-transfer/pull/645))
  - Feat/dt graphsync pullreqs (#627) ([filecoin-project/go-data-transfer#627](https://github.com/filecoin-project/go-data-transfer/pull/627))
  - fix(datatransfer): fix tests
  - Graphsync response is scheduled when a valid push request is received (#625) ([filecoin-project/go-data-transfer#625](https://github.com/filecoin-project/go-data-transfer/pull/625))
  - responses alert subscribers when request is not accepted (#607) ([filecoin-project/go-data-transfer#607](https://github.com/filecoin-project/go-data-transfer/pull/607))
  - feat(datatransfer): milestone 2 infrastructure
  - other tests passing
  - send data transfer response
  - a better reflection
  - remove unused fmt import in graphsync_test
  - cleanup for PR
  - tests passing
  - Initiate push and pull requests (#536) ([filecoin-project/go-data-transfer#536](https://github.com/filecoin-project/go-data-transfer/pull/536))
  - Fix tests
  - Respond to PR comments: * Make DataTransferRequest/Response be returned in from Net * Regenerate cbor_gen and fix the generator caller so it works better * Please the linters
  - Cleanup for PR, clarifying and additional comments
  - Some cleanup for PR
  - all message tests passing, some others in datatransfer
  - WIP trying out some stuff * Embed request/response in message so all the interfaces work AND the CBOR unmarshaling works: this is more like the spec anyway * get rid of pb stuff
  - * Bring cbor-gen stuff into datatransfer package * make transferRequest private struct * add transferResponse + funcs * Rename VoucherID to VoucherType * more tests passing
  - WIP using CBOR encoding for dataxfermsg
  - feat(datatransfer): setup implementation path
  - Duplicate comment ([filecoin-project/go-data-transfer#619](https://github.com/filecoin-project/go-data-transfer/pull/619))
  - fix typo ([filecoin-project/go-data-transfer#621](https://github.com/filecoin-project/go-data-transfer/pull/621))
  - fix types typo
  - refactor(datatransfer): implement style fixes
  - refactor(deals): move type instantiation to modules
  - refactor(datatransfer): xerrors, cbor-gen, tweaks
  - feat(datatransfer): make dag service dt async
  - refactor(datatransfer): add comments, renames
  - feat(datatransfer): integration w/ simple merkledag

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Shannon Wells | 12 | +4337/-3455 | 53 |
| hannahhoward | 20 | +5090/-1692 | 99 |
| shannonwells | 13 | +1720/-983 | 65 |
| Hannah Howard | 6 | +1393/-1262 | 45 |
| wanghui | 2 | +4/-4 | 2 |
| 郭光华 | 1 | +0/-1 | 1 |

### 🙌🏽 Want to contribute?

Would you like to contribute to this repo and don’t know how? Here are a few places you can get started:

- Check out the [Contributing Guidelines](https://github.com/filecoin-project/go-data-transfer/blob/master/CONTRIBUTING.md)
- Look for issues with the `good-first-issue` label in [go-fil-markets](https://github.com/filecoin-project/go-data-transfer/issues?utf8=%E2%9C%93&q=is%3Aissue+is%3Aopen+label%3A%22e-good-first-issue%22+)
