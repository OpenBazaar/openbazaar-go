package shared_testutil

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/network"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	chunk "github.com/ipfs/go-ipfs-chunker"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	files "github.com/ipfs/go-ipfs-files"
	ipldformat "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	unixfile "github.com/ipfs/go-unixfs/file"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/filecoin-project/go-storedcounter"
)

type Libp2pTestData struct {
	Ctx                     context.Context
	Ds1                     datastore.Batching
	Ds2                     datastore.Batching
	DTStoredCounter1        *storedcounter.StoredCounter
	DTStoredCounter2        *storedcounter.StoredCounter
	RetrievalStoredCounter1 *storedcounter.StoredCounter
	RetrievalStoredCounter2 *storedcounter.StoredCounter
	Bs1                     bstore.Blockstore
	Bs2                     bstore.Blockstore
	DagService1             ipldformat.DAGService
	DagService2             ipldformat.DAGService
	GraphSync1              graphsync.GraphExchange
	GraphSync2              graphsync.GraphExchange
	Loader1                 ipld.Loader
	Loader2                 ipld.Loader
	Storer1                 ipld.Storer
	Storer2                 ipld.Storer
	Host1                   host.Host
	Host2                   host.Host
	OrigBytes               []byte
}

func NewLibp2pTestData(ctx context.Context, t *testing.T) *Libp2pTestData {
	testData := &Libp2pTestData{}
	testData.Ctx = ctx
	makeLoader := func(bs bstore.Blockstore) ipld.Loader {
		return func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
			c, ok := lnk.(cidlink.Link)
			if !ok {
				return nil, errors.New("incorrect Link Type")
			}
			// read block from one store
			block, err := bs.Get(c.Cid)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(block.RawData()), nil
		}
	}

	makeStorer := func(bs bstore.Blockstore) ipld.Storer {
		return func(lnkCtx ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
			var buf bytes.Buffer
			var committer ipld.StoreCommitter = func(lnk ipld.Link) error {
				c, ok := lnk.(cidlink.Link)
				if !ok {
					return errors.New("incorrect Link Type")
				}
				block, err := blocks.NewBlockWithCid(buf.Bytes(), c.Cid)
				if err != nil {
					return err
				}
				return bs.Put(block)
			}
			return &buf, committer, nil
		}
	}
	testData.Ds1 = dss.MutexWrap(datastore.NewMapDatastore())
	testData.Ds2 = dss.MutexWrap(datastore.NewMapDatastore())

	testData.DTStoredCounter1 = storedcounter.New(testData.Ds1, datastore.NewKey("nextDTID"))
	testData.DTStoredCounter2 = storedcounter.New(testData.Ds2, datastore.NewKey("nextDTID"))

	testData.RetrievalStoredCounter1 = storedcounter.New(testData.Ds1, datastore.NewKey("nextDealID"))
	testData.RetrievalStoredCounter2 = storedcounter.New(testData.Ds2, datastore.NewKey("nextDealID"))

	// make a bstore and dag service
	testData.Bs1 = bstore.NewBlockstore(testData.Ds1)
	testData.Bs2 = bstore.NewBlockstore(testData.Ds2)

	testData.DagService1 = merkledag.NewDAGService(blockservice.New(testData.Bs1, offline.Exchange(testData.Bs1)))
	testData.DagService2 = merkledag.NewDAGService(blockservice.New(testData.Bs2, offline.Exchange(testData.Bs2)))

	// setup an IPLD loader/storer for bstore 1
	testData.Loader1 = makeLoader(testData.Bs1)
	testData.Storer1 = makeStorer(testData.Bs1)

	// setup an IPLD loader/storer for bstore 2
	testData.Loader2 = makeLoader(testData.Bs2)
	testData.Storer2 = makeStorer(testData.Bs2)

	mn := mocknet.New(ctx)

	// setup network
	var err error
	testData.Host1, err = mn.GenPeer()
	require.NoError(t, err)

	testData.Host2, err = mn.GenPeer()
	require.NoError(t, err)

	err = mn.LinkAll()
	require.NoError(t, err)

	testData.GraphSync1 = graphsyncimpl.New(ctx, network.NewFromLibp2pHost(testData.Host1), testData.Loader1, testData.Storer1)
	testData.GraphSync2 = graphsyncimpl.New(ctx, network.NewFromLibp2pHost(testData.Host2), testData.Loader2, testData.Storer2)

	return testData
}

const unixfsChunkSize uint64 = 1 << 10
const unixfsLinksPerLevel = 1024

// LoadUnixFSFile injects the fixture `filename` into the given blockstore from the
// fixtures directory. If useSecondNode is true, fixture is injected to the second node;
// otherwise the first node gets it
func (ltd *Libp2pTestData) LoadUnixFSFile(t *testing.T, fixturesPath string, useSecondNode bool) ipld.Link {

	// read in a fixture file
	fpath, err := filepath.Abs(filepath.Join(thisDir(t), "..", fixturesPath))
	require.NoError(t, err)

	f, err := os.Open(fpath)
	require.NoError(t, err)

	var buf bytes.Buffer
	tr := io.TeeReader(f, &buf)
	file := files.NewReaderFile(tr)

	// import to UnixFS
	var dagService ipldformat.DAGService
	if useSecondNode {
		dagService = ltd.DagService2
	} else {
		dagService = ltd.DagService1
	}
	bufferedDS := ipldformat.NewBufferedDAG(ltd.Ctx, dagService)

	params := helpers.DagBuilderParams{
		Maxlinks:   unixfsLinksPerLevel,
		RawLeaves:  true,
		CidBuilder: nil,
		Dagserv:    bufferedDS,
	}

	db, err := params.New(chunk.NewSizeSplitter(file, int64(unixfsChunkSize)))
	require.NoError(t, err)

	nd, err := balanced.Layout(db)
	require.NoError(t, err)

	err = bufferedDS.Commit()
	require.NoError(t, err)

	// save the original files bytes
	ltd.OrigBytes = buf.Bytes()

	return cidlink.Link{Cid: nd.Cid()}
}

func thisDir(t *testing.T) string {
	_, fname, _, ok := runtime.Caller(1)
	require.True(t, ok)
	return path.Dir(fname)
}

// VerifyFileTransferred checks that the fixture file was sent from one node to the other.
func (ltd *Libp2pTestData) VerifyFileTransferred(t *testing.T, link ipld.Link, useSecondNode bool, readLen uint64) {
	var dagService ipldformat.DAGService
	if useSecondNode {
		dagService = ltd.DagService2
	} else {
		dagService = ltd.DagService1
	}

	c := link.(cidlink.Link).Cid

	// load the root of the UnixFS DAG from the new blockstore
	otherNode, err := dagService.Get(ltd.Ctx, c)
	require.NoError(t, err)

	// Setup a UnixFS file reader
	n, err := unixfile.NewUnixfsFile(ltd.Ctx, dagService, otherNode)
	require.NoError(t, err)

	fn, ok := n.(files.File)
	require.True(t, ok)

	// Read the bytes for the UnixFS File
	finalBytes := make([]byte, readLen)
	_, err = fn.Read(finalBytes)
	if err != nil {
		require.Equal(t, "EOF", err.Error())
	}

	// verify original bytes match final bytes!
	require.EqualValues(t, ltd.OrigBytes[:readLen], finalBytes)
}
