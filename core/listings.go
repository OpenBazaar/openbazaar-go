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
	profile, err := n.GetProfile()
	if err != nil {
		return c, err
	}
	p := new(pb.ID_Pubkeys)
	p.Guid = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return c, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	id.BlockchainID = profile.Handle
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

	// set to zero as the inventory counts shouldn't be seeded with the listing
	listing.InventoryCount = 0
	for _, option := range listing.Item.Options {
		for _, variant := range option.Variants {
			variant.InventoryCount = 0
		}
	}
	c.VendorListings = append(c.VendorListings, listing)
	c.Signatures = append(c.Signatures, s)
	return c, nil
}

func (n *OpenBazaarNode) SetListingInventory(listing *pb.Listing) error {
	if len(listing.Item.Options) == 0 {
		return n.Datastore.Inventory().Put(listing.Slug, int(listing.InventoryCount))
	}
	for _, option := range listing.Item.Options {
		for _, variant := range option.Variants {
			err := n.Datastore.Inventory().Put(path.Join(listing.Slug, option.Title, variant.Name), int(variant.InventoryCount))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Update the index.json file in the listings directory
func (n *OpenBazaarNode) UpdateListingIndex(contract *pb.RicardianContract) error {
	type price struct {
		CurrencyCode string
		Price        float64
	}
	type listingData struct {
		Hash      string
		Slug      string
		Title     string
		Category  []string
		ItemType  string
		Desc      string
		Thumbnail string
		Price     price
	}
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	listingPath := path.Join(n.RepoPath, "root", "listings", contract.VendorListings[0].Slug, "listing.json")

	var index []listingData

	listingHash, err := ipfs.AddFile(n.Context, listingPath)
	if err != nil {
		return err
	}

	ld := listingData{
		Hash:      listingHash,
		Slug:      contract.VendorListings[0].Slug,
		Title:     contract.VendorListings[0].Item.Title,
		Category:  contract.VendorListings[0].Item.Categories,
		ItemType:  contract.VendorListings[0].Metadata.ItemType.String(),
		Desc:      contract.VendorListings[0].Item.Description[:140],
		Thumbnail: contract.VendorListings[0].Item.Images[0].Hash,
		Price:     price{contract.VendorListings[0].Item.Price.CurrencyCode, contract.VendorListings[0].Item.Price.Price},
	}

	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// read existing file
		file, err := ioutil.ReadFile(indexPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(file, &index)
		if err != nil {
			return err
		}
	}

	// Check to see if the listing we are adding already exists in the list. If so delete it.
	for i, d := range index {
		if d.Slug != ld.Slug {
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
	defer f.Close()
	if err != nil {
		return err
	}

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

func (n *OpenBazaarNode) GetListingCount() int {
	type price struct {
		CurrencyCode string
		Price        float64
	}
	type listingData struct {
		Hash      string
		Slug      string
		Title     string
		Category  []string
		ItemType  string
		Desc      string
		Thumbnail string
		Price     price
	}
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")

	// read existing file
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return 0
	}

	var index []listingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return 0
	}
	return len(index)
}

func validate(listing *pb.Listing) error {
	// TODO: validate this listing to make sure all values are correct
	return nil
}
