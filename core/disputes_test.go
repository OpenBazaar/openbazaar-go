package core_test

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcutil"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/OpenBazaar/wallet-interface"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"

	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/OpenBazaar/wallet-interface/mocks"
)

func TestOpenBazaarDisputes_SignDispute(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}
	contract := new(pb.RicardianContract)
	_, err = node.SignDispute(contract)
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
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}

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

	mockWallet := new(mocks.Wallet)
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
	//contract.BuyerOrder.Payment.Coin = "TBTC"

	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err != nil {
		t.Error("Expired contract was able to be disputed:", err)
	}

	fmt.Println(node.Datastore.Sales().GetByOrderId("12345"))

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
