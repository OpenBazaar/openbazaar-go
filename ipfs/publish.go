package ipfs

import (
	"errors"

	"context"
	"fmt"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	coreCmds "github.com/ipfs/go-ipfs/core/commands"
	"github.com/ipfs/go-ipfs/namesys"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"gx/ipfs/QmPR2JzfKd9poHx9XBhzoFeBBC31ZM3W5iUPKJZWyaoZZm/go-libp2p-routing"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	dhtpb "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record/pb"
	"time"
)

var pubErr = errors.New(`Name publish failed`)

// Publish a signed IPNS record to our Peer ID
func Publish(ctx commands.Context, hash string) (string, error) {
	args := []string{"name", "publish", "/ipfs/" + hash}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)
	resp := res.Output()
	if res.Error() != nil {
		log.Error(res.Error())
		return "", res.Error()
	}
	returnedVal := resp.(*coreCmds.IpnsEntry).Value
	if returnedVal != "/ipfs/"+hash {
		return "", pubErr
	}
	log.Infof("Published %s to IPNS", hash)
	return returnedVal, nil
}

// Publish another IPFS record at /ipns/<peerID>:<altRoot>
func PublishAltRoot(ctx commands.Context, altRoot string, value path.Path, eol time.Time) error {
	nd, err := ctx.ConstructNode()
	if err != nil {
		return err
	}
	hash, err := mh.FromB58String(nd.Identity.Pretty())
	if err != nil {
		return err
	}
	ipnskey := "/ipns/" + string(hash) + ":" + altRoot

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
		// None found, lets start at zero!
		return 0, err
	}
	var val []byte
	if err == nil {
		prbytes, ok := prevrec.([]byte)
		if !ok {
			return 0, fmt.Errorf("unexpected type returned from datastore: %#v", prevrec)
		}
		dhtrec := new(dhtpb.Record)
		err := proto.Unmarshal(prbytes, dhtrec)
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

	entry, err := namesys.CreateRoutingEntryData(k, value, seqnum, eol)
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
