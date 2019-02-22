package core_test

import (
	"fmt"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/openbazaar-go/test"

	"github.com/OpenBazaar/openbazaar-go/core"

	"github.com/btcsuite/btcutil"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/OpenBazaar/wallet-interface"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"

	"github.com/OpenBazaar/wallet-interface/mocks"
)

var node *core.OpenBazaarNode

func TestMain(m *testing.M) {

	setUp()
	retCode := m.Run()
	tearDown()

	// call with result of m.Run()
	os.Exit(retCode)

}

func setUp() {
	// Setup offline messaging outbox
	err := os.MkdirAll("./tmp/outbox", os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}

	node, err = test.NewNode()
	if err != nil {
		return
	}

	storage := selfhosted.NewSelfHostedStorage("./tmp", node.IpfsNode, []peer.ID{}, func(peerID string, cids []cid.Cid) error { return nil })
	node.MessageStorage = storage
}

func tearDown() {
	os.RemoveAll("./tmp")
}

func TestOpenBazaarDisputes_SignDispute(t *testing.T) {

	contract := new(pb.RicardianContract)
	_, err := node.SignDispute(contract)
	if err == nil {
		t.Fail()
	}

	contract.Dispute = new(pb.Dispute)
	_, err = node.SignDispute(contract)
	if err != nil {
		t.Fail()
	}
}

func TestOpenBazaarDisputes_VerifyEscrowFundsAreDisputable(t *testing.T) {

	// Test Invalid contract with no listings
	contract := factory.NewDisputeableContract()
	transactionRecord := new(wallet.TransactionRecord)
	transactionRecord.Txid = "INVALIDHEX"

	records := []*wallet.TransactionRecord{transactionRecord}
	disputable := node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Contract with no vendor listings disputable")
	}

	// Add listing to the contract
	newListing := factory.NewListing("TEST")
	newListing.Metadata.EscrowTimeoutHours = 1
	newListing.Metadata.CoinType = "BTC"

	contract.BuyerOrder.Payment.Coin = "XXXXXXXXXXX" // Invalid coin type
	contract.VendorListings = []*pb.Listing{newListing}

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Invalid wallet type was found")
	}

	contract.BuyerOrder.Payment.Coin = "XXX"

	mockWallet := &mocks.Wallet{}
	node.Multiwallet[100000] = mockWallet

	mockWallet.On("CurrencyCode").Return("XXX")

	// Disputable validation
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a842"
	disputablehash, _ := chainhash.NewHashFromStr(records[0].Txid)
	mockWallet.On("GetConfirmations", *disputablehash).Return(uint32(5), uint32(5), nil)

	addr, err := btcutil.DecodeAddress("3FZrcR7enANkjZRdKJ5vhaLNBwibZvTP9w", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}

	mockWallet.On("CurrentAddress", wallet.EXTERNAL).Return(addr)

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if !disputable {
		t.Error("Valid contract was not disputable")
	}

	contract.BuyerOrder.Payment.Moderator = "QmfU2ELKbhTG5515rF18F6nSLKuySDK47YKDfHWxjsbT7v"
	contract.BuyerOrder.BuyerID.PeerID = "QmfU2ELKbhTG5515rF18F6nSLKuySDK47YKDfHWxjsbT7x"

	// Test opening a dispute on a sale
	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err != nil {
		t.Error("Expired contract was able to be disputed:", err)
	}

	// Ensure dispute got attached to the contract
	disputedSaleContract, _, _, _, _, _, err := node.Datastore.Sales().GetByOrderId("12345")
	if err != nil {
		t.Error("Could not retrieve disputed sale", err)
	}
	if disputedSaleContract.Dispute == nil {
		t.Error("Sale contract does not have a dispute attached")
	}

	// Test opening a dispute on a purchase
	contract.BuyerOrder.BuyerID.PeerID = node.IpfsNode.Identity.Pretty()

	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err != nil {
		t.Error("Expired contract was able to be disputed:", err)
	}
	disputedPurchaseContract, _, _, _, _, _, err := node.Datastore.Purchases().GetByOrderId("12345")
	if err != nil {
		t.Error("Could not retrieve disputed purchase", err)
	}
	if disputedPurchaseContract.Dispute == nil {
		t.Error("Purchase contract does not have a dispute attached")
	}

	// Expired test
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a843"
	expiredhash, _ := chainhash.NewHashFromStr(records[0].Txid)
	mockWallet.On("GetConfirmations", *expiredhash).Return(uint32(7), uint32(7), nil)
	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Expired contract was able to be disputed")
	}

	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err == nil {
		t.Error("Expired contract was able to be disputed")
	}

}

func TestOpenBazaarDisputes_VerifySignatureOnDisputeOpen(t *testing.T) {
	contract := factory.NewDisputeableContract()
	peerID := contract.BuyerOrder.BuyerID.PeerID

	// Set up transaction records
	transactionRecord := new(wallet.TransactionRecord)
	records := []*wallet.TransactionRecord{transactionRecord}

	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a842"
	//disputablehash, _ := chainhash.NewHashFromStr(records[0].Txid)

	err := node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(contract.Dispute)

	node.VerifySignatureOnDisputeOpen(contract, peerID)
}
