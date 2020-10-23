package multistore

import (
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-filestore"
	"github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	ipldprime "github.com/ipld/go-ipld-prime"
)

// Store is a single store instance returned by the MultiStore.
// it gives public access to the blockstore, filestore, dag service,
// and an ipld-prime loader/storer
type Store struct {
	ds datastore.Batching

	fm     *filestore.FileManager
	Fstore *filestore.Filestore

	Bstore blockstore.Blockstore

	bsvc   blockservice.BlockService
	DAG    ipld.DAGService
	Loader ipldprime.Loader
	Storer ipldprime.Storer
}

func openStore(ds datastore.Batching) (*Store, error) {
	blocks := namespace.Wrap(ds, datastore.NewKey("blocks"))
	bs := blockstore.NewBlockstore(blocks)

	fm := filestore.NewFileManager(ds, "/")
	fm.AllowFiles = true

	fstore := filestore.NewFilestore(bs, fm)
	ibs := blockstore.NewIdStore(fstore)

	bsvc := blockservice.New(ibs, offline.Exchange(ibs))
	dag := merkledag.NewDAGService(bsvc)

	loader := storeutil.LoaderForBlockstore(ibs)
	storer := storeutil.StorerForBlockstore(ibs)

	return &Store{
		ds: ds,

		fm:     fm,
		Fstore: fstore,

		Bstore: ibs,

		bsvc:   bsvc,
		DAG:    dag,
		Loader: loader,
		Storer: storer,
	}, nil
}

// Close closes down the blockservice used by the DAG Service for this store
func (s *Store) Close() error {
	return s.bsvc.Close()
}
