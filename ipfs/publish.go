package ipfs

import (
	"context"
	"errors"
	"gx/ipfs/QmaRFtZhVAwXBk4Z3zEsvjScH9fjsDZmhXfa1Gm8eMb9cg/go-ipns"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"gx/ipfs/QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt/go-ipfs-ds-help"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	dhtpb "gx/ipfs/Qma9Eqp16mNHDX1EL73pcxhFfzbyXVcAYtaDd1xdmDRDtL/go-libp2p-record/pb"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"

	"gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"
	"time"

	"gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
	pb "gx/ipfs/QmaRFtZhVAwXBk4Z3zEsvjScH9fjsDZmhXfa1Gm8eMb9cg/go-ipns/pb"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("ipfs")

var pubErr = errors.New(`Name publish failed`)

// Publish a signed IPNS record to our Peer ID
func Publish(n *core.IpfsNode, hash string) error {
	err := n.Namesys.Publish(context.Background(), n.PrivateKey, path.FromString("/ipfs/"+hash))
	if err == nil {
		log.Infof("Published %s to IPNS", hash)
		return nil
	} else {
		return pubErr
	}
}

// Publish another IPFS record at /ipns/<peerID>:<altRoot>
func PublishAltRoot(nd *core.IpfsNode, altRoot string, value path.Path, eol time.Time) error {
	ipnskey := "/ipns/" + nd.Identity.Pretty() + ":" + altRoot

	// get previous records sequence number
	seqnum, err := getPreviousSeqNo(context.Background(), nd, ipnskey)
	if err != nil {
		return err
	}

	// increment it
	seqnum++

	return PutRecordToRouting(context.Background(), ipnskey, nd.PrivateKey, value, seqnum, eol, nd.Routing)
}

func getPreviousSeqNo(ctx context.Context, nd *core.IpfsNode, ipnskey string) (uint64, error) {
	prevrec, err := nd.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if err != nil && err != ds.ErrNotFound {
		return 0, err
	}
	var val []byte
	if err == nil {
		dhtrec := new(dhtpb.Record)
		err := proto.Unmarshal(prevrec, dhtrec)
		if err != nil {
			return 0, err
		}

		val = dhtrec.GetValue()
	} else {
		// try and check the dht for a record
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()

		rv, err := nd.Routing.GetValue(ctx, ipnskey)
		if err != nil {
			// no such record found, start at zero!
			return 0, nil
		}

		val = rv
	}

	e := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, e)
	if err != nil {
		return 0, err
	}

	return e.GetSequence(), nil
}

func PutRecordToRouting(ctx context.Context, ipnskey string, k ci.PrivKey, value path.Path, seqnum uint64, eol time.Time, r routing.ValueStore) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	entry, err := ipns.Create(k, []byte(value), seqnum, eol)
	if err != nil {
		return err
	}

	ttl, ok := checkCtxTTL(ctx)
	if ok {
		entry.Ttl = proto.Uint64(uint64(ttl.Nanoseconds()))
	}

	errs := make(chan error, 1)

	go func() {
		errs <- namesys.PublishEntry(ctx, r, ipnskey, entry)
	}()

	return waitOnErrChan(ctx, errs)
}

func waitOnErrChan(ctx context.Context, errs chan error) error {
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func checkCtxTTL(ctx context.Context) (time.Duration, bool) {
	v := ctx.Value("ipns-publish-ttl")
	if v == nil {
		return 0, false
	}

	d, ok := v.(time.Duration)
	return d, ok
}
