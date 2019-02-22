package core_test

import (
	"fmt"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
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
var testModerator = "QmfU2ELKbhTG5515rF18F6nSLKuySDK47YKDfHWxjsbT7v"
var testVendor = "QmfU2ELKbhTG5515rF18F6nSLKuySDK47YKDfHWxjsbT7y"

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
	contract.BuyerOrder.Payment.Moderator = testModerator // Add moderator
	contract.VendorListings = []*pb.Listing{}             // Remove vendorListings
	records := []*wallet.TransactionRecord{nil}

	disputable := node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Contract with no vendor listings disputable")
	}

	// Test invalid hex for txid
	transactionRecord := new(wallet.TransactionRecord)
	transactionRecord.Txid = "INVALIDHEX"
	records = []*wallet.TransactionRecord{transactionRecord}

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Contract the includes txids with invalid hex encoding allowed")
	}

	// Test invalid payment coin for non-existent wallet
	newListing := factory.NewListing("TEST LISTING")
	contract.BuyerOrder.Payment.Coin = "XXXXXXXXXXX" // Invalid coin type
	contract.VendorListings = []*pb.Listing{newListing}

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Invalid wallet type was identified as valid")
	}

	// Create mock wallet for testing
	mockWallet := &mocks.Wallet{}

	// Create mock transaction
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a842"
	txHash, _ := chainhash.NewHashFromStr(records[0].Txid)

	// Create mock current address
	addr, err := btcutil.DecodeAddress("3FZrcR7enANkjZRdKJ5vhaLNBwibZvTP9w", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}

	// Mock wallet methods
	mockWallet.On("GetConfirmations", *txHash).Return(uint32(5), uint32(5), nil)
	mockWallet.On("CurrencyCode").Return("XXX")
	mockWallet.On("CurrentAddress", wallet.EXTERNAL).Return(addr)

	node.Multiwallet[100000] = mockWallet

	contract.BuyerOrder.Payment.Coin = "XXX"
	contract.VendorListings[0].Metadata.EscrowTimeoutHours = 1 // Require six confirmations

	// Test for valid dispute
	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if !disputable {
		t.Error("Valid contract was not disputable")
	}

	// Test expired sale disputability
	contract.BuyerOrder.BuyerID.PeerID = "QmfU2ELKbhTG5515rF18F6nSLKuySDK47YKDfHWxjsbT7x"

	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err != nil {
		t.Error("Expired contract was able to be disputed:", err)
	}

	// Test retrieving a sales contract
	disputedSaleContract, _, _, _, _, _, err := node.Datastore.Sales().GetByOrderId("12345")
	if err != nil {
		t.Error("Could not retrieve disputed sale", err)
	}

	// Test that the dispute was opened properly
	if disputedSaleContract.Dispute == nil {
		t.Error("Sale contract does not have a dispute attached")
	}

	// Test opening a dispute on a purchase
	contract.BuyerOrder.BuyerID.PeerID = node.IpfsNode.Identity.Pretty()

	// Test expired purchase disputability
	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err != nil {
		t.Error("Expired contract was able to be disputed:", err)
	}

	// Test retrieving a purchase contract
	disputedPurchaseContract, _, _, _, _, _, err := node.Datastore.Purchases().GetByOrderId("12345")
	if err != nil {
		t.Error("Could not retrieve disputed purchase", err)
	}

	// Test that the dispute was opened properly
	if disputedPurchaseContract.Dispute == nil {
		t.Error("Purchase contract does not have a dispute attached")
	}

	// Test that an expired contract isn't flagged as disputable
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a843"
	expiredHash, _ := chainhash.NewHashFromStr(records[0].Txid)
	mockWallet.On("GetConfirmations", *expiredHash).Return(uint32(7), uint32(7), nil)

	disputable = node.VerifyEscrowFundsAreDisputable(contract, records)
	if disputable {
		t.Error("Expired contract was able to be disputed")
	}

	// Test opening a dispute on an expired contract
	err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
	if err == nil {
		t.Error("Expired contract was able to be disputed")
	}

}

func TestOpenBazaarDisputes_VerifySignatureOnDisputeOpen(t *testing.T) {
	contract := factory.NewDisputeableContract()
	contract.BuyerOrder.Payment.Coin = "XXX"
	contract.VendorListings[0].Metadata.EscrowTimeoutHours = 1

	identityKey, _ := node.IpfsNode.PrivateKey.GetPublic().Bytes()
	contract.BuyerOrder.BuyerID.Pubkeys.Identity = identityKey

	contract.BuyerOrder.BuyerID.PeerID = node.IpfsNode.Identity.Pretty()
	peerID := contract.BuyerOrder.BuyerID.PeerID
	contract.BuyerOrder.Payment.Moderator = testModerator
	contract.VendorListings[0].VendorID.PeerID = testVendor

	// Create mock wallet for testing
	mockWallet := &mocks.Wallet{}

	// Create mock current address
	addr, err := btcutil.DecodeAddress("3FZrcR7enANkjZRdKJ5vhaLNBwibZvTP9w", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}

	// Mock wallet methods
	mockWallet.On("CurrencyCode").Return("XXX")
	mockWallet.On("CurrentAddress", wallet.EXTERNAL).Return(addr)

	node.Multiwallet[100000] = mockWallet

	// Set up transaction records
	transactionRecord := new(wallet.TransactionRecord)
	records := []*wallet.TransactionRecord{transactionRecord}
	records[0].Txid = "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a842"
	txHash, _ := chainhash.NewHashFromStr(records[0].Txid)
	mockWallet.On("GetConfirmations", *txHash).Return(uint32(5), uint32(5), nil)

	if node.VerifyEscrowFundsAreDisputable(contract, records) {
		err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
		if err != nil {
			t.Error(err)
		}

		// Test empty dispute
		badContract := new(pb.RicardianContract)
		err = node.VerifySignatureOnDisputeOpen(badContract, peerID)
		if err == nil {
			t.Error(err)
		}

		// Test serialized contract in dispute has listings
		vendorListings := contract.VendorListings
		contract.VendorListings = []*pb.Listing{}
		contract.Dispute.SerializedContract, err = core.GetSerializedContract(contract)
		if err != nil {
			t.Error(err)
		}
		err = node.VerifySignatureOnDisputeOpen(contract, peerID)
		if err == nil {
			t.Error(err)
		}

		// Test serialized contract in dispute has buyer order
		contract.VendorListings = vendorListings
		buyerOrder := contract.BuyerOrder
		contract.BuyerOrder = new(pb.Order)
		contract.Dispute.SerializedContract, err = core.GetSerializedContract(contract)
		if err != nil {
			t.Error(err)
		}

		err = node.VerifySignatureOnDisputeOpen(contract, peerID)
		if err == nil {
			t.Error(err)
		}

		// Test serialized contract has no matching peer IDs
		contract.BuyerOrder = buyerOrder
		buyerPeer := contract.BuyerOrder.BuyerID.PeerID
		vendorPeer := contract.VendorListings[0].VendorID.PeerID
		contract.BuyerOrder.BuyerID.PeerID = "TEST"
		contract.VendorListings[0].VendorID.PeerID = "TEST"
		contract.Dispute.SerializedContract, err = core.GetSerializedContract(contract)
		if err != nil {
			t.Error(err)
		}

		err = node.VerifySignatureOnDisputeOpen(contract, peerID)
		if err == nil {
			t.Error(err)
		}

		contract.BuyerOrder.BuyerID.PeerID = buyerPeer
		contract.VendorListings[0].VendorID.PeerID = vendorPeer
		contract.Dispute.SerializedContract, err = core.GetSerializedContract(contract)
		if err != nil {
			t.Error(err)
		}

		// Test bad signature on the contract
		err = node.VerifySignatureOnDisputeOpen(contract, peerID)
		if err == nil {
			t.Error(err)
		}

		// Test valid signature on dispute
		err = node.OpenDispute("12345", contract, records, "TEST CLAIM")
		if err != nil {
			t.Error(err)
		}
		err = node.VerifySignatureOnDisputeOpen(contract, contract.BuyerOrder.BuyerID.PeerID)
		if err != nil {
			t.Error(err)
		}

		// Test no signatures on the contract
		contract.Signatures = []*pb.Signature{}

		err = node.VerifySignatureOnDisputeOpen(contract, peerID)
		if err == nil {
			t.Error(err)
		}

	}

}
