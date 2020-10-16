package multiwallet

import (
	"fmt"
	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/multiwallet/filecoin"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/op/go-logging"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestMultiWallet_Filecoin(t *testing.T) {
	mdb := datastore.NewMockMultiwalletDatastore()
	db, err := mdb.GetDatastoreForWallet(wallet.Filecoin)
	if err != nil {
		t.Fatal(err)
	}

	logger := logging.NewLogBackend(os.Stdout, "", 0)

	cfg := &config.Config{
		Mnemonic: "abcdefg",
		Params:   &chaincfg.MainNetParams,
		Cache:    cache.NewMockCacher(),
		Coins: []config.CoinConfig{
			{
				CoinType:   wallet.Filecoin,
				DB:         db,
				ClientAPIs: []string{"http://localhost:8080/api"},
			},
		},
		Logger: logger,
	}

	w, err := NewMultiWallet(cfg)
	if err != nil {
		t.Fatal(err)
	}

	w.Start()

	fmt.Println(w[wallet.Filecoin].CurrentAddress(wallet.EXTERNAL))

	<-time.After(time.Second * 40)

	addr, err := filecoin.NewFilecoinAddress("t3vjuvunjquznv6nlhs72utwndnr6xlaaqf3xeympz4bj4cclxtldrdlcdqvdx2fragwlo6xddd475uezjeapq")
	if err != nil {
		t.Fatal(err)
	}

	txid, err := w[wallet.Filecoin].Spend(*big.NewInt(10000), addr, wallet.NORMAL, "", false)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(txid)
	select {}
}
