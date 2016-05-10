package core

import (
	"crypto/sha256"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	ec "github.com/btcsuite/btcd/btcec"
)

func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.RicardianContract, error) {
	c := new(pb.RicardianContract)
	if err := validate(listing); err != nil {
		return c, err
	}
	listing.VendorID.Guid = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return c, err
	}
	listing.VendorID.Pubkeys.Guid = pubkey
	listing.VendorID.Pubkeys.Bitcoin = n.Wallet.GetMasterPublicKey().Key
	s := new(pb.Signatures)
	s.Section = pb.Signatures_LISTING
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		return c, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedListing)
	if err != nil {
		return c, err
	}
	priv, _ := ec.PrivKeyFromBytes(ec.S256(), n.Wallet.GetMasterPrivateKey().Key)
	hashed := sha256.Sum256(serializedListing)
	bitcoinSig, err := priv.Sign(hashed[:])
	if err != nil {
		return c, err
	}
	s.Guid = guidSig
	s.Bitcoin = bitcoinSig.Serialize()
	c.VendorListing = append(c.VendorListing, listing)
	c.Signatures = append(c.Signatures, s)
	return c, nil
}

func validate(listing *pb.Listing) error {
	// TODO: validate this listing to make sure all values are correct
	return nil
}