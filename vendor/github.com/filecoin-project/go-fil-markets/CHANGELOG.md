# go-fil-markets changelog

# go-fil-markets 0.1.0

Initial tagged release for Filecoin Testnet Phase 2

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(release): document release process (#206) ([filecoin-project/go-fil-markets#206](https://github.com/filecoin-project/go-fil-markets/pull/206))
  - update types_cbor_gen (#203) ([filecoin-project/go-fil-markets#203](https://github.com/filecoin-project/go-fil-markets/pull/203))
  - Upgrade to specs-actors v0.2.0 (#204) ([filecoin-project/go-fil-markets#204](https://github.com/filecoin-project/go-fil-markets/pull/204))
  - Storagemarket/provider allows subscription to events (#202) ([filecoin-project/go-fil-markets#202](https://github.com/filecoin-project/go-fil-markets/pull/202))
  - Add a test rule to Makefile, use in CI config (#200) ([filecoin-project/go-fil-markets#200](https://github.com/filecoin-project/go-fil-markets/pull/200))
  - Update to specs-actors v1.0.0 (#198) ([filecoin-project/go-fil-markets#198](https://github.com/filecoin-project/go-fil-markets/pull/198))
  - add multiple peers per payloadCID (#197) ([filecoin-project/go-fil-markets#197](https://github.com/filecoin-project/go-fil-markets/pull/197))
  - refactor(storedcounter): use extracted package (#196) ([filecoin-project/go-fil-markets#196](https://github.com/filecoin-project/go-fil-markets/pull/196))
  - Feat/no block chain ops (#190) ([filecoin-project/go-fil-markets#190](https://github.com/filecoin-project/go-fil-markets/pull/190))
  - Add a max piece size to storage asks (#188) ([filecoin-project/go-fil-markets#188](https://github.com/filecoin-project/go-fil-markets/pull/188))
  - Update proofs to v25 params (#189) ([filecoin-project/go-fil-markets#189](https://github.com/filecoin-project/go-fil-markets/pull/189))
  - Update Graphsync (#184) ([filecoin-project/go-fil-markets#184](https://github.com/filecoin-project/go-fil-markets/pull/184))
  - Support selectors on retrieval (#187) ([filecoin-project/go-fil-markets#187](https://github.com/filecoin-project/go-fil-markets/pull/187))
  - Add optional PieceCID to block unsealing (#186) ([filecoin-project/go-fil-markets#186](https://github.com/filecoin-project/go-fil-markets/pull/186))
  - Add Selector to retrieval params (#175) ([filecoin-project/go-fil-markets#175](https://github.com/filecoin-project/go-fil-markets/pull/175))
  - use PieceCID if provided in QueryParams (#181) ([filecoin-project/go-fil-markets#181](https://github.com/filecoin-project/go-fil-markets/pull/181))
  - include rejection reason in client response (#182) ([filecoin-project/go-fil-markets#182](https://github.com/filecoin-project/go-fil-markets/pull/182))
  - Do not create CAR file when propsing a storage deal using Manual Transfer (#183) ([filecoin-project/go-fil-markets#183](https://github.com/filecoin-project/go-fil-markets/pull/183))
  - add selector to BlockIO classes (#178) ([filecoin-project/go-fil-markets#178](https://github.com/filecoin-project/go-fil-markets/pull/178))
  - rename list deals interface & impls (#174) ([filecoin-project/go-fil-markets#174](https://github.com/filecoin-project/go-fil-markets/pull/174))
  - Feat/configure start epoch buffer (#171) ([filecoin-project/go-fil-markets#171](https://github.com/filecoin-project/go-fil-markets/pull/171))
  - send tipset identifier to node when interacting with chain (#172) ([filecoin-project/go-fil-markets#172](https://github.com/filecoin-project/go-fil-markets/pull/172))
  - Support Retrieval By Any CID, Not Just Root (#166) ([filecoin-project/go-fil-markets#166](https://github.com/filecoin-project/go-fil-markets/pull/166))
  - v24 groth parameters (#167) ([filecoin-project/go-fil-markets#167](https://github.com/filecoin-project/go-fil-markets/pull/167))
  - Add TipSetToken to SavePaymentVoucher (#165) ([filecoin-project/go-fil-markets#165](https://github.com/filecoin-project/go-fil-markets/pull/165))
  - retrieval client node interface passes tipset identifier to node (#164) ([filecoin-project/go-fil-markets#164](https://github.com/filecoin-project/go-fil-markets/pull/164))
  - send state identifiery when getting miner worker address (#153) ([filecoin-project/go-fil-markets#153](https://github.com/filecoin-project/go-fil-markets/pull/153))
  - chore(deps): update to ipld/go-car (#152) ([filecoin-project/go-fil-markets#152](https://github.com/filecoin-project/go-fil-markets/pull/152))
  - add TipSet identity-producing method to various Node interfaces (#149) ([filecoin-project/go-fil-markets#149](https://github.com/filecoin-project/go-fil-markets/pull/149))
  - conform imports to schema defined in CONTRIBUTING.md (#150) ([filecoin-project/go-fil-markets#150](https://github.com/filecoin-project/go-fil-markets/pull/150))
  - Refactor Storage Provider to FSM Module (#145) ([filecoin-project/go-fil-markets#145](https://github.com/filecoin-project/go-fil-markets/pull/145))
  - Feat/update to fix 32gib verification (#147) ([filecoin-project/go-fil-markets#147](https://github.com/filecoin-project/go-fil-markets/pull/147))
  - ci(codecov): remove cbor gen files from coverage
  - ci(codecov): ignore cbor gen files (#146) ([filecoin-project/go-fil-markets#146](https://github.com/filecoin-project/go-fil-markets/pull/146))
  - Storage Client Statemachine Refactor (#136) ([filecoin-project/go-fil-markets#136](https://github.com/filecoin-project/go-fil-markets/pull/136))
  - upgrade to libfilecoin version that supports cache clearing (#138) ([filecoin-project/go-fil-markets#138](https://github.com/filecoin-project/go-fil-markets/pull/138))
  - fix(cborgen): update cbor gen for dataref (#137) ([filecoin-project/go-fil-markets#137](https://github.com/filecoin-project/go-fil-markets/pull/137))
  - allow manual piece commitment (#135) ([filecoin-project/go-fil-markets#135](https://github.com/filecoin-project/go-fil-markets/pull/135))
  - fix(retrievalmarket): handle self-retrieval correctly (#134) ([filecoin-project/go-fil-markets#134](https://github.com/filecoin-project/go-fil-markets/pull/134))
  - feat(retrievalmarket): support wallet address (#130) ([filecoin-project/go-fil-markets#130](https://github.com/filecoin-project/go-fil-markets/pull/130))
  - allow specification of 'wallet' for ensure funds calls (#129) ([filecoin-project/go-fil-markets#129](https://github.com/filecoin-project/go-fil-markets/pull/129))
  - update to filecoin-ffi with shared types (#127) ([filecoin-project/go-fil-markets#127](https://github.com/filecoin-project/go-fil-markets/pull/127))
  - feat(sharedcounter): persist counter to disk (#125) ([filecoin-project/go-fil-markets#125](https://github.com/filecoin-project/go-fil-markets/pull/125))
  - Use go-statemachine + FSMs in retrieval market (#124) ([filecoin-project/go-fil-markets#124](https://github.com/filecoin-project/go-fil-markets/pull/124))
  - storage client: Call EnsureFunds more correctly (#123) ([filecoin-project/go-fil-markets#123](https://github.com/filecoin-project/go-fil-markets/pull/123))
  - use latest specs-actors with uint64 lane and nonce from paych.Actor (#122) ([filecoin-project/go-fil-markets#122](https://github.com/filecoin-project/go-fil-markets/pull/122))
  - Update go-sectorbuilder to latest that uses specs-actors types (#121) ([filecoin-project/go-fil-markets#121](https://github.com/filecoin-project/go-fil-markets/pull/121))
  - Import spec actor types (#118) ([filecoin-project/go-fil-markets#118](https://github.com/filecoin-project/go-fil-markets/pull/118))
  - Update README (#120) ([filecoin-project/go-fil-markets#120](https://github.com/filecoin-project/go-fil-markets/pull/120))
  - chore(cborgen): update cborgen
  - Merge branch 'head/lotus' into lotus/merge-02-10-2020
  - Storage Market integration test (#119) ([filecoin-project/go-fil-markets#119](https://github.com/filecoin-project/go-fil-markets/pull/119))
  - fix(storagemarket): add back in cid recording (#115) ([filecoin-project/go-fil-markets#115](https://github.com/filecoin-project/go-fil-markets/pull/115))
  - fix(storagemarket): assign net member (#114) ([filecoin-project/go-fil-markets#114](https://github.com/filecoin-project/go-fil-markets/pull/114))
  - Fix/flaky tests (#113) ([filecoin-project/go-fil-markets#113](https://github.com/filecoin-project/go-fil-markets/pull/113))
  - Storage market network abstraction (#109) ([filecoin-project/go-fil-markets#109](https://github.com/filecoin-project/go-fil-markets/pull/109))
  - Remove Sector ID from MinerDeal (merge from head/lotus -- PLEASE USE MERGE COMMIT) ([filecoin-project/go-fil-markets#112](https://github.com/filecoin-project/go-fil-markets/pull/112))
  - No Filestore On Storage Client (#107) ([filecoin-project/go-fil-markets#107](https://github.com/filecoin-project/go-fil-markets/pull/107))
  - take miner address as parameter (#108) ([filecoin-project/go-fil-markets#108](https://github.com/filecoin-project/go-fil-markets/pull/108))
  - skip flaky 1 block tests (#104) ([filecoin-project/go-fil-markets#104](https://github.com/filecoin-project/go-fil-markets/pull/104))
  - use go-padreader instead of local copy (#103) ([filecoin-project/go-fil-markets#103](https://github.com/filecoin-project/go-fil-markets/pull/103))
  - Handle sector id in the `OnDealSectorCommitted` callback (#58) ([filecoin-project/go-fil-markets#58](https://github.com/filecoin-project/go-fil-markets/pull/58))
  - Properly Implement Retrieval Lookups Based on CIDs (#57) ([filecoin-project/go-fil-markets#57](https://github.com/filecoin-project/go-fil-markets/pull/57))
  - Add Stop funcs to retrieval providers (#56) ([filecoin-project/go-fil-markets#56](https://github.com/filecoin-project/go-fil-markets/pull/56))
  - refactor(retrievalmarket): switch to payload CIDs (#55) ([filecoin-project/go-fil-markets#55](https://github.com/filecoin-project/go-fil-markets/pull/55))
  - Move to an explicit piecestore and explicit unsealing. (#54) ([filecoin-project/go-fil-markets#54](https://github.com/filecoin-project/go-fil-markets/pull/54))
  - Improve test coverage, fix any bugs (#53) ([filecoin-project/go-fil-markets#53](https://github.com/filecoin-project/go-fil-markets/pull/53))
  - Techdebt/1 block file retrieval test (#51) ([filecoin-project/go-fil-markets#51](https://github.com/filecoin-project/go-fil-markets/pull/51))
  - ci(config): use large resource_class (#52) ([filecoin-project/go-fil-markets#52](https://github.com/filecoin-project/go-fil-markets/pull/52))
  - Sync up DealState to match spec (#50) ([filecoin-project/go-fil-markets#50](https://github.com/filecoin-project/go-fil-markets/pull/50))
  - Support arbitrary dag retrieval (#46) ([filecoin-project/go-fil-markets#46](https://github.com/filecoin-project/go-fil-markets/pull/46))
  - RetrievalMarket: Query + Deal integration test, + bug fixes uncovered during writing the test (#36) ([filecoin-project/go-fil-markets#36](https://github.com/filecoin-project/go-fil-markets/pull/36))
  - Remove filestore as a go between with StorageMiner, pass direct io.reader to StorageMiner (#49) ([filecoin-project/go-fil-markets#49](https://github.com/filecoin-project/go-fil-markets/pull/49))
  - Feat/find providers (#43) ([filecoin-project/go-fil-markets#43](https://github.com/filecoin-project/go-fil-markets/pull/43))
  - Retrieval Deals, Spec V0 (#37) ([filecoin-project/go-fil-markets#37](https://github.com/filecoin-project/go-fil-markets/pull/37))
  - Lotus updates ([filecoin-project/go-fil-markets#45](https://github.com/filecoin-project/go-fil-markets/pull/45))
  - storagemarket: close channel on return (#42) ([filecoin-project/go-fil-markets#42](https://github.com/filecoin-project/go-fil-markets/pull/42))
  - Feat/verify data before publishing deal (#40) ([filecoin-project/go-fil-markets#40](https://github.com/filecoin-project/go-fil-markets/pull/40))
  - Use CAR and padding for piece data (#27) ([filecoin-project/go-fil-markets#27](https://github.com/filecoin-project/go-fil-markets/pull/27))
  - Upgrade Query Protocol to Spec V0 (#25) ([filecoin-project/go-fil-markets#25](https://github.com/filecoin-project/go-fil-markets/pull/25))
  - Merge branch 'lotus-updates'
  - fix(retrievalmarket): add mutex around subscribers (#32) (#33) ([filecoin-project/go-fil-markets#33](https://github.com/filecoin-project/go-fil-markets/pull/33))
  - ci(codecov): disable status, display report (#31) ([filecoin-project/go-fil-markets#31](https://github.com/filecoin-project/go-fil-markets/pull/31))
  - Flaky test fix (#28) ([filecoin-project/go-fil-markets#28](https://github.com/filecoin-project/go-fil-markets/pull/28))
  - skip flaky test (#30) ([filecoin-project/go-fil-markets#30](https://github.com/filecoin-project/go-fil-markets/pull/30))
  - Network Abstraction For Retrieval Market (#17) ([filecoin-project/go-fil-markets#17](https://github.com/filecoin-project/go-fil-markets/pull/17))
  - Use CAR file in generation of CommP (#26) ([filecoin-project/go-fil-markets#26](https://github.com/filecoin-project/go-fil-markets/pull/26))
  - filestore: track close err, lints (#20) ([filecoin-project/go-fil-markets#20](https://github.com/filecoin-project/go-fil-markets/pull/20))
  - Deleting datatransfer files (#19) ([filecoin-project/go-fil-markets#19](https://github.com/filecoin-project/go-fil-markets/pull/19))
  - Use shared go-filecoin packages go-cbor-util, go-address, go-crypto, (#22) ([filecoin-project/go-fil-markets#22](https://github.com/filecoin-project/go-fil-markets/pull/22))
  - Storage Market Extraction (#15) ([filecoin-project/go-fil-markets#15](https://github.com/filecoin-project/go-fil-markets/pull/15))
  - Retrieval Market Extraction (#13) ([filecoin-project/go-fil-markets#13](https://github.com/filecoin-project/go-fil-markets/pull/13))
  - PieceIO improvements (#12) ([filecoin-project/go-fil-markets#12](https://github.com/filecoin-project/go-fil-markets/pull/12))
  - fix links in datatransfer README (#11) ([filecoin-project/go-fil-markets#11](https://github.com/filecoin-project/go-fil-markets/pull/11))
  - fix(build): fix tools build error (#14) ([filecoin-project/go-fil-markets#14](https://github.com/filecoin-project/go-fil-markets/pull/14))
  - fix(tokenamount): fix naming (#10) ([filecoin-project/go-fil-markets#10](https://github.com/filecoin-project/go-fil-markets/pull/10))
  - feat(shared): add shared tools and types (#9) ([filecoin-project/go-fil-markets#9](https://github.com/filecoin-project/go-fil-markets/pull/9))
  - add circle config, let's ci ([filecoin-project/go-fil-markets#7](https://github.com/filecoin-project/go-fil-markets/pull/7))
  - Skeleton readme ([filecoin-project/go-fil-markets#5](https://github.com/filecoin-project/go-fil-markets/pull/5))
  - Feat/datatransfer readme, contributing, design doc (rename)
  - Piece IO ([filecoin-project/go-fil-markets#2](https://github.com/filecoin-project/go-fil-markets/pull/2))
  - Feat/datatransfer graphsync movein ([filecoin-project/go-fil-markets#1](https://github.com/filecoin-project/go-fil-markets/pull/1))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 38 | +27080/-10375 | 455 |
| Ingar Shu | 10 | +1315/-6870 | 127 |
| shannonwells | 12 | +5500/-70 | 48 |
| Shannon Wells | 20 | +2671/-940 | 109 |
| ergastic | 4 | +1835/-501 | 47 |
| Erin Swenson-Healey | 9 | +516/-408 | 112 |
| hannahhoward | 10 | +497/-150 | 79 |
| Łukasz Magiera | 4 | +379/-139 | 19 |
| whyrusleeping | 3 | +239/-87 | 19 |
| Whyrusleeping | 4 | +192/-96 | 26 |
| Aayush Rajasekaran | 3 | +93/-13 | 14 |
| Mosh | 2 | +37/-8 | 2 |
| Ignacio Hagopian | 2 | +9/-11 | 2 |
| Alex North | 2 | +11/-7 | 4 |
| Alex Cruikshank | 1 | +1/-9 | 1 |

# go-fil-markets 0.1.1

Hotfix release for spec actors update

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - chore(changelog): update changelog for tagged release
  - Upgrade to specs-actors v0.3.0 (#207) ([filecoin-project/go-fil-markets#207](https://github.com/filecoin-project/go-fil-markets/pull/207))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| hannahhoward | 1 | +9/-1 | 1 |
| Alex North | 1 | +3/-3 | 2 |

# go-fil-markets 0.1.2

Hotfix release for transitive dependencies to use new go-ipld-prime

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): update changelog
  - Upgrade IPLD-prime to latest (#215) ([filecoin-project/go-fil-markets#215](https://github.com/filecoin-project/go-fil-markets/pull/215))
- github.com/filecoin-project/go-data-transfer (v0.0.0-20200408061858-82c58b423ca6 -> v0.2.0):
  - Upgrade graphsync + ipld-prime (#49) ([filecoin-project/go-data-transfer#49](https://github.com/filecoin-project/go-data-transfer/pull/49))
  - Use extracted generic pubsub (#48) ([filecoin-project/go-data-transfer#48](https://github.com/filecoin-project/go-data-transfer/pull/48))
  - Refactor & Cleanup In Preparation For Added Complexity (#47) ([filecoin-project/go-data-transfer#47](https://github.com/filecoin-project/go-data-transfer/pull/47))
  - feat(graphsync): complete notifications for responder (#46) ([filecoin-project/go-data-transfer#46](https://github.com/filecoin-project/go-data-transfer/pull/46))
- github.com/ipfs/go-graphsync (v0.0.6-0.20200408061628-e1a98fc64c42 -> v0.0.6-0.20200428204348-97a8cf76a482):
  - refactor(hooks): use external pubsub (#65) ([ipfs/go-graphsync#65](https://github.com/ipfs/go-graphsync/pull/65))
  - Update of IPLD Prime (#66) ([ipfs/go-graphsync#66](https://github.com/ipfs/go-graphsync/pull/66))
  - Add standard issue template
  - feat(responsemanager): add listener for completed responses (#64) ([ipfs/go-graphsync#64](https://github.com/ipfs/go-graphsync/pull/64))
  - Update Requests (#63) ([ipfs/go-graphsync#63](https://github.com/ipfs/go-graphsync/pull/63))
  - Add pausing and unpausing of requests (#62) ([ipfs/go-graphsync#62](https://github.com/ipfs/go-graphsync/pull/62))
  - ci(circle): remove benchmark task for now
  - ci(circle): update orb

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 10 | +5409/-4023 | 151 |
| Hector Sanjuan | 1 | +27/-0 | 2 |
| hannahhoward | 3 | +16/-8 | 5 |


# go-fil-markets 0.1.3

Hotfix release for critical graphsync bug fix

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): add release documentation
  - fix(deps): update to tagged data transfer
  - chore(deps): update data transfer + graphsync
- github.com/filecoin-project/go-data-transfer (v0.2.0 -> v0.2.1):
  - chore(deps): update graphsync
- github.com/ipfs/go-graphsync (v0.0.6-0.20200428204348-97a8cf76a482 -> v0.0.6-0.20200504202014-9d5f2c26a103):
  - fix(responsemanager): add nil check (#67) ([ipfs/go-graphsync#67](https://github.com/ipfs/go-graphsync/pull/67))
  - Add autocomment configuration

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hector Sanjuan | 1 | +68/-0 | 1 |
| hannahhoward | 4 | +20/-12 | 7 |
| Hannah Howard | 1 | +4/-0 | 1 |

# go-fil-markets 0.2.0

Asynchronous operations release -- we no longer synchronously wait for chain messages to push

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): update changelog for 0.2.0 release
  - Storage Market Changes Based On Lotus Integration (#223) ([filecoin-project/go-fil-markets#223](https://github.com/filecoin-project/go-fil-markets/pull/223))
  - Merge in hotfix 0.1.3 ([filecoin-project/go-fil-markets#225](https://github.com/filecoin-project/go-fil-markets/pull/225))
  - ppl can sub to storage client evts (#217) ([filecoin-project/go-fil-markets#217](https://github.com/filecoin-project/go-fil-markets/pull/217))
  - fix(storagemarket): set miner peer id on deals (#216) ([filecoin-project/go-fil-markets#216](https://github.com/filecoin-project/go-fil-markets/pull/216))
  - chore(release): merge hotfix 0.1.2 branch back
  - docs(release): update release process (#212) ([filecoin-project/go-fil-markets#212](https://github.com/filecoin-project/go-fil-markets/pull/212))
  - Nonblocking storage deals [#80] (#194) ([filecoin-project/go-fil-markets#194](https://github.com/filecoin-project/go-fil-markets/pull/194))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Ingar Shu | 1 | +993/-608 | 13 |
| Hannah Howard | 3 | +101/-59 | 14 |
| Shannon Wells | 1 | +106/-31 | 5 |
| hannahhoward | 1 | +8/-0 | 1 |

# go-fil-markets 0.2.1

Hotfix release -- updates to try to solve deal stream problems attempt #1

### Changelog
- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): update for 0.2.1 release
  - update to v26 proofs (#232) ([filecoin-project/go-fil-markets#232](https://github.com/filecoin-project/go-fil-markets/pull/232))
  - Don't Keep Streams Open (#230) ([filecoin-project/go-fil-markets#230](https://github.com/filecoin-project/go-fil-markets/pull/230))
  - Round-trip storage/retrieval test (#229) ([filecoin-project/go-fil-markets#229](https://github.com/filecoin-project/go-fil-markets/pull/229))
  - feat(storagemarket): improve human readable constant maps (#228) ([filecoin-project/go-fil-markets#228](https://github.com/filecoin-project/go-fil-markets/pull/228))
  - fix(deps): update data-transfer 0.3.0 (#227) ([filecoin-project/go-fil-markets#227](https://github.com/filecoin-project/go-fil-markets/pull/227))
  - docs(CHANGELOG): update changelog for 0.2.0 release ([filecoin-project/go-fil-markets#226](https://github.com/filecoin-project/go-fil-markets/pull/226))
- github.com/filecoin-project/go-data-transfer (v0.2.1 -> v0.3.0):
  - feat(graphsyncimpl): fix open/close events (#52) ([filecoin-project/go-data-transfer#52](https://github.com/filecoin-project/go-data-transfer/pull/52))
  - chore(deps): update graphsync ([filecoin-project/go-data-transfer#51](https://github.com/filecoin-project/go-data-transfer/pull/51))
  - Refactor registry and encoding (#50) ([filecoin-project/go-data-transfer#50](https://github.com/filecoin-project/go-data-transfer/pull/50))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 5 | +1841/-1303 | 59 |
| Shannon Wells | 1 | +511/-141 | 19 |
| hannahhoward | 1 | +11/-1 | 1 |
| Erin Swenson-Healey | 1 | +1/-1 | 1 |

# go-fil-markets 0.2.2

Hotfix release -- updates to try to solve deal stream problems attempt #2 & v26 params update

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): docs for 0.2.2 release
  - feat(storagemarket): revert protocol changes (#236) ([filecoin-project/go-fil-markets#236](https://github.com/filecoin-project/go-fil-markets/pull/236))
  - Feat/cbor gen check ci #231 (#234) ([filecoin-project/go-fil-markets#234](https://github.com/filecoin-project/go-fil-markets/pull/234))
  - update sector-storage and break transitive dependency on lotus (#235) ([filecoin-project/go-fil-markets#235](https://github.com/filecoin-project/go-fil-markets/pull/235))
  - docs(CHANGELOG): update for 0.2.1 release ([filecoin-project/go-fil-markets#233](https://github.com/filecoin-project/go-fil-markets/pull/233))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +701/-614 | 22 |
| Erin Swenson-Healey | 1 | +5/-265 | 2 |
| Shannon Wells | 1 | +11/-0 | 1 |
| hannahhoward | 1 | +8/-1 | 1 |

# go-fil-markets 0.2.3

Hotfix release -- final fix for issues with deal streams held open

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - feat(CHANGELOG): update changelog for v0.2.3
  - feat(network): tag connections to preserve them (#246) ([filecoin-project/go-fil-markets#246](https://github.com/filecoin-project/go-fil-markets/pull/246))
  - docs(CHANGELOG): docs for 0.2.2 release ([filecoin-project/go-fil-markets#243](https://github.com/filecoin-project/go-fil-markets/pull/243))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +112/-7 | 10 |
| hannahhoward | 1 | +7/-1 | 1 |

# go-fil-markets 0.2.4

go-filecoin compatibility release

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): update change log
  - Buffer the done channel when adding storage collateral (#249) ([filecoin-project/go-fil-markets#249](https://github.com/filecoin-project/go-fil-markets/pull/249))
  - feat(CHANGELOG): update changelog for v0.2.3 ([filecoin-project/go-fil-markets#248](https://github.com/filecoin-project/go-fil-markets/pull/248))
  - Unified request validator (#247) ([filecoin-project/go-fil-markets#247](https://github.com/filecoin-project/go-fil-markets/pull/247))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Ingar Shu | 2 | +221/-230 | 7 |
| hannahhoward | 1 | +8/-0 | 1 |

# go-fil-markets 0.2.5

go-filecoin compatibility release

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): update for 0.2.5 release
  - Fixes from filecoin integration work (#253) ([filecoin-project/go-fil-markets#253](https://github.com/filecoin-project/go-fil-markets/pull/253))
  - docs(CHANGELOG): update change log ([filecoin-project/go-fil-markets#250](https://github.com/filecoin-project/go-fil-markets/pull/250))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +138/-68 | 7 |
| hannahhoward | 1 | +8/-3 | 3 |

# go-fil-markets 0.2.6

Remove data store wrapping

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - Feat/change prefixes 256 (#257) ([filecoin-project/go-fil-markets#257](https://github.com/filecoin-project/go-fil-markets/pull/257))
  - docs(CHANGELOG): update for 0.2.5 release ([filecoin-project/go-fil-markets#254](https://github.com/filecoin-project/go-fil-markets/pull/254))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Shannon Wells | 1 | +6/-15 | 5 |

# go-fil-markets 0.2.7

Custom Deal Decision Logic and cleanups of 0.2.6

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - docs(CHANGELOG): update changelog for 0.2.7
  - refactor(storagemarket): remove storedask from provider (#263) ([filecoin-project/go-fil-markets#263](https://github.com/filecoin-project/go-fil-markets/pull/263))
  - Deal Decision Custom Function (#262) ([filecoin-project/go-fil-markets#262](https://github.com/filecoin-project/go-fil-markets/pull/262))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 2 | +142/-27 | 11 |
| shannonwells | 1 | +19/-6 | 1 |

# go-fil-markets 0.3.0

Deal Resumability release. We now attempt to resume storage deals when the application is shut down and restart, and we support a more flexible deal acceptance protocol.

### Changelog
- github.com/filecoin-project/go-fil-markets:
  - fix(storagemarket): fix validator, add to test
  - docs(CHANGELOG): update changelog and add detail script
  - both StoredAsk and storage Provider are scoped to a single miner (#276) ([filecoin-project/go-fil-markets#276](https://github.com/filecoin-project/go-fil-markets/pull/276))
  - specs actors v0.6 (#274) ([filecoin-project/go-fil-markets#274](https://github.com/filecoin-project/go-fil-markets/pull/274))
  - Restartable storage deals (#270) ([filecoin-project/go-fil-markets#270](https://github.com/filecoin-project/go-fil-markets/pull/270))
  - replace AddAsk with SetAsk, to convey intent (#275) ([filecoin-project/go-fil-markets#275](https://github.com/filecoin-project/go-fil-markets/pull/275))
  - Allow custom decisioning for a provider to decide retrieval deals. (#269) ([filecoin-project/go-fil-markets#269](https://github.com/filecoin-project/go-fil-markets/pull/269))
  - Feat/module docs #83 (#267) ([filecoin-project/go-fil-markets#267](https://github.com/filecoin-project/go-fil-markets/pull/267))
  - Tentative acceptance protocol (#244) ([filecoin-project/go-fil-markets#244](https://github.com/filecoin-project/go-fil-markets/pull/244))
  - docs(CHANGELOG): update changelog for 0.2.7 ([filecoin-project/go-fil-markets#264](https://github.com/filecoin-project/go-fil-markets/pull/264))
- github.com/filecoin-project/go-statemachine (v0.0.0-20200226041606-2074af6d51d9 -> v0.0.0-20200612181802-4eb3d0c68eba):
  - Serialize notifications (#11) ([filecoin-project/go-statemachine#11](https://github.com/filecoin-project/go-statemachine/pull/11))
  - Run callback in goroutine (#10) ([filecoin-project/go-statemachine#10](https://github.com/filecoin-project/go-statemachine/pull/10))
  - Finality States ([filecoin-project/go-statemachine#9](https://github.com/filecoin-project/go-statemachine/pull/9))
  - Documentation, particularly for FSM Module (#8) ([filecoin-project/go-statemachine#8](https://github.com/filecoin-project/go-statemachine/pull/8))
  - Call stageDone on nil nextStep ([filecoin-project/go-statemachine#7](https://github.com/filecoin-project/go-statemachine/pull/7))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Ingar Shu | 4 | +1407/-695 | 35 |
| Shannon Wells | 2 | +1515/-467 | 20 |
| hannahhoward | 8 | +862/-191 | 21 |
| Hannah Howard | 1 | +263/-0 | 2 |
| Łukasz Magiera | 1 | +48/-43 | 15 |
| Erin Swenson-Healey | 2 | +39/-42 | 10 |

# go-fil-markets 0.3.1

Hotfix release to get `use addresses from miner info for connecting to miners` task merged for downstream dependencies to used

### Changelog
- github.com/filecoin-project/go-fil-markets:
  - use addresses from miner info for connecting to miners (#290) ([filecoin-project/go-fil-markets#290](https://github.com/filecoin-project/go-fil-markets/pull/290))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Whyrusleeping | 1 | +53/-5 | 9 |

# go-fil-markets 0.3.1.1

Hotfix bug release to address critical issues affecting node startup

### Changelog

- github.com/filecoin-project/go-fil-markets:
  - add locks protecting retrieval market maps (#311) ([filecoin-project/go-fil-markets#311](https://github.com/filecoin-project/go-fil-markets/pull/311))
  - fix(storagemarket): run deal restarts in go routine (#309) ([filecoin-project/go-fil-markets#309](https://github.com/filecoin-project/go-fil-markets/pull/309))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Hannah Howard | 1 | +13/-7 | 2 |
| vyzo | 1 | +10/-0 | 1 |

### 🙌🏽 Want to contribute?

Would you like to contribute to this repo and don’t know how? Here are a few places you can get started:

- Check out the [Contributing Guidelines](https://github.com/filecoin-project/go-fil-markets/blob/master/CONTRIBUTING.md)
- Look for issues with the `good-first-issue` label in [go-fil-markets](https://github.com/filecoin-project/go-fil-markets/issues?utf8=%E2%9C%93&q=is%3Aissue+is%3Aopen+label%3A%22e-good-first-issue%22+)
