package core_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

func TestOpenBazaarContract_ContractHasListings(t *testing.T) {
	contract := new(pb.RicardianContract)
	if core.HasListings(contract) {
		t.Error("Contract does not actually have any listings")
	}

	contract.VendorListings = []*pb.Listing{
		{Slug: "slug"},
	}
	if !core.HasListings(contract) {
		t.Error("Contract has listings")
	}

}

func TestOpenBazaarContract_ContractHasBuyerOrder(t *testing.T) {
	contract := new(pb.RicardianContract)
	if core.ContractHasBuyerOrder(contract) {
		t.Error("Contract does not actually have a buyer order")
	}

	contract.BuyerOrder = new(pb.Order)
	contract.BuyerOrder.RefundAddress = "TESTADDRESS"
	if !core.ContractHasBuyerOrder(contract) {
		t.Error("Contract has a buyer order")
	}

}

func TestOpenBazaarContract_ContractIsMissingVendorID(t *testing.T) {
	contract := new(pb.RicardianContract)
	if !core.ContractIsMissingVendorID(contract) {
		t.Error("Contract does not actually have a vendor ID")
	}

	contract.VendorListings = []*pb.Listing{
		{Slug: "slug"},
	}
	contract.VendorListings[0].VendorID = new(pb.ID)

	if !core.ContractIsMissingVendorID(contract) {
		t.Error("Contract has a vendor ID but no pubkeys")
	}

	contract.VendorListings[0].VendorID.Pubkeys = new(pb.ID_Pubkeys)
	if core.ContractIsMissingVendorID(contract) {
		t.Error("Contract has a vendor ID")
	}

}

func TestOpenBazaarContract_GetContractOrderType(t *testing.T) {
	contract := new(pb.RicardianContract)

	contract.BuyerOrder = &pb.Order{
		Payment: &pb.Order_Payment{
			Moderator: "TEST",
		},
		BuyerID: &pb.ID{
			PeerID: "TEST2",
		},
	}

	contract.VendorListings = []*pb.Listing{
		{VendorID: &pb.ID{
			PeerID: "TEST3"},
		},
	}

	role := core.GetContractOrderType(contract, "TEST")
	if role != "case" {
		t.Error("Misidentified case")
	}

	role = core.GetContractOrderType(contract, "TEST2")
	if role != "purchase" {
		t.Error("Misidentified case")
	}

	role = core.GetContractOrderType(contract, "TEST3")
	if role != "sale" {
		t.Error("Misidentified case")
	}

}
