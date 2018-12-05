package cmd

import (
	"time"

	dht "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht"

	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap/network"

	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

// Configure the node's bootstrap peers and protocol tags for testnet
func setupTestnet(cfgData []byte) error {
	testnetBootstrapAddrs, err := schema.GetTestnetBootstrapAddrs(cfgData)
	if err != nil {
		log.Error(err)
		return err
	}

	nodeCfg, err := nodeRepo.Config()
	if err != nil {
		log.Error("Could not retrieve IPFS config", err)
		return err
	}

	nodeCfg.Bootstrap = testnetBootstrapAddrs
	dht.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
	bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
	service.ProtocolOpenBazaar = "/openbazaar/app/testnet/1.0.0"

	cfg.dataSharingConfig.PushTo = []string{}

	return nil
}

func setTestmodeRecordAgingIntervals() {
	repo.VendorDisputeTimeout_lastInterval = time.Duration(60) * time.Minute

	repo.ModeratorDisputeExpiry_firstInterval = time.Duration(20) * time.Minute
	repo.ModeratorDisputeExpiry_secondInterval = time.Duration(40) * time.Minute
	repo.ModeratorDisputeExpiry_thirdInterval = time.Duration(59) * time.Minute
	repo.ModeratorDisputeExpiry_lastInterval = time.Duration(60) * time.Minute

	repo.BuyerDisputeTimeout_firstInterval = time.Duration(20) * time.Minute
	repo.BuyerDisputeTimeout_secondInterval = time.Duration(40) * time.Minute
	repo.BuyerDisputeTimeout_thirdInterval = time.Duration(59) * time.Minute
	repo.BuyerDisputeTimeout_lastInterval = time.Duration(60) * time.Minute
	repo.BuyerDisputeTimeout_totalDuration = time.Duration(60) * time.Minute

	repo.BuyerDisputeExpiry_firstInterval = time.Duration(20) * time.Minute
	repo.BuyerDisputeExpiry_secondInterval = time.Duration(40) * time.Minute
	repo.BuyerDisputeExpiry_lastInterval = time.Duration(59) * time.Minute
	repo.BuyerDisputeExpiry_totalDuration = time.Duration(60) * time.Minute
}
