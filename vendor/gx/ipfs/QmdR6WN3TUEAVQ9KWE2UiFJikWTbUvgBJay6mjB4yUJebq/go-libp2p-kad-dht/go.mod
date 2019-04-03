module github.com/libp2p/go-libp2p-kad-dht

require (
	github.com/btcsuite/btcd v0.0.0-20190213025234-306aecffea32 // indirect
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/fd/go-nat v1.0.0 // indirect
	github.com/go-check/check v0.0.0-20180628173108-788fd7840127 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/google/uuid v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.0
	github.com/ipfs/go-cid v0.9.0
	github.com/ipfs/go-datastore v3.2.0+incompatible
	github.com/ipfs/go-detect-race v1.0.1 // indirect
	github.com/ipfs/go-ipfs-util v1.2.8
	github.com/ipfs/go-log v1.5.7
	github.com/ipfs/go-todocounter v1.0.1
	github.com/jbenet/go-cienv v0.0.0-20150120210510-1bb1476777ec // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99
	github.com/jbenet/go-randbuf v0.0.0-20160322125720-674640a50e6a // indirect
	github.com/jbenet/go-temp-err-catcher v0.0.0-20150120210811-aac704a3f4f2 // indirect
	github.com/jbenet/goprocess v0.0.0-20160826012719-b497e2f366b8
	github.com/kr/pretty v0.1.0 // indirect
	github.com/libp2p/go-addr-util v0.0.0-20190221001233-73d4c93d8ab2 // indirect
	github.com/libp2p/go-buffer-pool v0.0.0-20190123225638-a8d831235797 // indirect
	github.com/libp2p/go-conn-security v0.0.0-20190218175802-3e30d86de3d7 // indirect
	github.com/libp2p/go-conn-security-multistream v0.0.0-20190218181649-199bde7312d5 // indirect
	github.com/libp2p/go-flow-metrics v0.0.0-20180906182756-7e5a55af4853 // indirect
	github.com/libp2p/go-libp2p v0.0.0-20190221041601-695d0ce80195
	github.com/libp2p/go-libp2p-blankhost v0.0.0-20190221000941-4dbe4842fd96 // indirect
	github.com/libp2p/go-libp2p-crypto v0.0.0-20190218135128-e333f2201582
	github.com/libp2p/go-libp2p-host v0.0.0-20190218184026-1a71c422ef28
	github.com/libp2p/go-libp2p-interface-connmgr v0.0.0-20190218180940-c8552ddb959e // indirect
	github.com/libp2p/go-libp2p-interface-pnet v0.0.0-20180919000501-d240acf619f6 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.0.0-20190218185511-f98f2bd87bdf
	github.com/libp2p/go-libp2p-loggables v0.0.0-20190218142206-5b80b7ea4ee3 // indirect
	github.com/libp2p/go-libp2p-metrics v0.0.0-20190218143726-eb0033e81c5e // indirect
	github.com/libp2p/go-libp2p-nat v0.0.0-20190218144058-a304452f6e87 // indirect
	github.com/libp2p/go-libp2p-net v0.0.0-20190222114911-e37f4ea19d2f
	github.com/libp2p/go-libp2p-netutil v0.0.0-20190218181806-719d15bce148 // indirect
	github.com/libp2p/go-libp2p-peer v0.0.0-20190225225425-9b0c59cc3369
	github.com/libp2p/go-libp2p-peerstore v0.0.0-20190222144347-c123410c6409
	github.com/libp2p/go-libp2p-protocol v0.0.0-20171212212132-b29f3d97e3a2
	github.com/libp2p/go-libp2p-record v0.0.0-20190218150535-4e8ffc3e2485
	github.com/libp2p/go-libp2p-routing v0.0.0-20190221041536-330243f43148
	github.com/libp2p/go-libp2p-secio v0.0.0-20190218175819-38f90b017ad1 // indirect
	github.com/libp2p/go-libp2p-swarm v0.0.0-20190219121359-7a03ca822298
	github.com/libp2p/go-libp2p-transport v0.0.0-20190218175832-d2bc1c17e028 // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.0.0-20190218180826-68cf0192f1d4 // indirect
	github.com/libp2p/go-maddr-filter v0.0.0-20181224014115-666a1351c131 // indirect
	github.com/libp2p/go-mplex v0.0.0-20190218180303-8ac902b6abdf // indirect
	github.com/libp2p/go-msgio v0.0.0-20190117001650-f8aaa1f70c8b // indirect
	github.com/libp2p/go-reuseport-transport v0.0.0-20190226153717-a4b1f2833c68 // indirect
	github.com/libp2p/go-stream-muxer v0.0.0-20190218175335-a3f82916c8ad // indirect
	github.com/libp2p/go-tcp-transport v0.0.0-20190218180853-ddf3e5c50ef0 // indirect
	github.com/libp2p/go-testutil v0.0.0-20190218143632-25ef001b4017
	github.com/mattn/go-colorable v0.1.1 // indirect
	github.com/mr-tron/base58 v1.1.0
	github.com/multiformats/go-multiaddr v0.0.1
	github.com/multiformats/go-multiaddr-dns v0.0.0-20181204224821-b3d6340f0777
	github.com/multiformats/go-multibase v0.0.0-20190219024939-f25b77813c0a // indirect
	github.com/multiformats/go-multistream v0.0.0-20181023014559-0c61f185f3d6
	github.com/stretchr/testify v1.3.0
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/go-notifier v0.0.0-20170827234753-097c5d47330f // indirect
	github.com/whyrusleeping/go-smux-multiplex v3.0.16+incompatible // indirect
	github.com/whyrusleeping/go-smux-multistream v2.0.2+incompatible // indirect
	github.com/whyrusleeping/go-smux-yamux v2.0.8+incompatible // indirect
	github.com/whyrusleeping/mafmt v1.2.8 // indirect
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7 // indirect
	github.com/whyrusleeping/yamux v1.1.5 // indirect
	golang.org/x/net v0.0.0-20190225153610-fe579d43d832 // indirect
)
