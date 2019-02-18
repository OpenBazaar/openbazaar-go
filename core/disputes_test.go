package core_test

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/OpenBazaar/wallet-interface"

	"github.com/OpenBazaar/openbazaar-go/pb"

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
	signed, err := node.SignDispute(contract)
	if err != nil {
		t.Fail()
	}
	fmt.Println(signed)
}

func TestOpenBazaarDisputes_VerifyEscrowFundsAreDisputable(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}

	// Test Invalid contract with no listings
	contract := new(pb.RicardianContract)
	transactionRecord := new(wallet.TransactionRecord)
	transactionRecord.Txid = "INVALIDHEX"

	records := []*wallet.TransactionRecord{transactionRecord}
	disputable := node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Fail()
	}

	newListing := new(pb.Listing)
	newMetadata := new(pb.Listing_Metadata)
	newMetadata.EscrowTimeoutHours = 1
	newListing.Metadata = newMetadata

	buyerOrder := new(pb.Order)
	buyerOrderPayment := new(pb.Order_Payment)
	buyerOrderPayment.Coin = "XXXXXXXXXX" // Invalid coin type
	buyerOrder.Payment = buyerOrderPayment

	contract.VendorListings = []*pb.Listing{newListing}
	contract.BuyerOrder = buyerOrder

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Fail()
	}

	contract.BuyerOrder.Payment.Coin = "OPENBAZAAR"

	mockWallet := new(mocks.Wallet)
	node.Multiwallet[100000] = mockWallet

	mockWallet.On("CurrencyCode").Return("OPENBAZAAR")

	// Disputable validation
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a842"
	disputablehash, _ := chainhash.NewHashFromStr(records[0].Txid)
	mockWallet.On("GetConfirmations", *disputablehash).Return(uint32(5), uint32(5), nil)

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if !disputable {
		t.Fail()
	}

	// Expired test
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a843"
	expiredhash, _ := chainhash.NewHashFromStr(records[0].Txid)
	mockWallet.On("GetConfirmations", *expiredhash).Return(uint32(7), uint32(7), nil)
	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Fail()
	}

}
