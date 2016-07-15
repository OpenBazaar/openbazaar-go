package core

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"os"
	"path"
)

// Add our identity to the listings and sign it
func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.RicardianContract, error) {
	c := new(pb.RicardianContract)
	if err := validate(listing); err != nil {
		return c, err
	}
	id := new(pb.ID)
	id.Guid = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return c, err
	}
	//TODO: add blockchain ID to listing
	p := new(pb.ID_Pubkeys)
	p.Guid = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return c, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	listing.VendorID = id
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
	priv, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return c, err
	}
	hashed := sha256.Sum256(serializedListing)
	bitcoinSig, err := priv.Sign(hashed[:])
	if err != nil {
		return c, err
	}
	s.Guid = guidSig
	s.Bitcoin = bitcoinSig.Serialize()
	c.VendorListings = append(c.VendorListings, listing)
	c.Signatures = append(c.Signatures, s)
	return c, nil
}

// Update the index.json file in the listings directory
func (n *OpenBazaarNode) UpdateListingIndex(contract *pb.RicardianContract) error {
	type listingData struct {
		Hash string
		Name string
	}
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	listingPath := path.Join(n.RepoPath, "root", "listings", contract.VendorListings[0].ListingName, "listing.json")

	// read existing file
	file, _ := ioutil.ReadFile(indexPath)
	listingHash, err := ipfs.AddFile(n.Context, listingPath)
	if err != nil {
		return err
	}
	ld := listingData{
		Hash: listingHash,
		Name: contract.VendorListings[0].ListingName,
	}

	var index []listingData
	json.Unmarshal(file, &index)

	// Check to see if the listing we are adding already exists in the list. If so delete it.
	for i, d := range index {
		if d.Name != ld.Name {
			continue
		}

		if len(index) == 1 {
			index = []listingData{}
			break
		}
		index = append(index[:i], index[i+1:]...)
	}

	// Append our listing with the new hash to the list
	index = append(index, ld)

	// write it back to file
	f, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := f.Write(j)
	if werr != nil {
		return werr
	}
	return nil
}

func validate(listing *pb.Listing) error {
	// TODO: validate this listing to make sure all values are correct
	return nil
}
