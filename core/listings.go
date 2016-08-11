package core

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"os"
	"path"
	"time"
	"fmt"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
)

const ListingVersion = 1
const TitleMaxCharacters = 140
const DescriptionMaxCharacters = 50000
const MaxTags = 10
const CategoryMaxCharacters = 70
const TagMaxCharacters = 70

// Add our identity to the listings and sign it
func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.RicardianContract, error) {
	c := new(pb.RicardianContract)
	// Check the listing data is correct for continuing
	if err := validate(listing); err != nil {
		return c, err
	}

	// Set listing version
	listing.Metadata.Version = ListingVersion

	// Add the vendor id to the listing
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
	id.BlockchainID = profile.Handle
	p := new(pb.ID_Pubkeys)
	p.Guid = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return c, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	listing.VendorID = id

	// Sign listing
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
		Desc:      contract.VendorListings[0].Item.Description[:160],
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
	if listing.Slug == "" {
		return errors.New("Slug must not be nil")
	}
	if int(listing.Metadata.ListingType) == 0 ||  int(listing.Metadata.ListingType) > 2 {
		return errors.New("Invalid listing type")
	}
	if int(listing.Metadata.ItemType) == 0 ||  int(listing.Metadata.ItemType) > 3 {
		return errors.New("Invalid item type")
	}
	if time.Unix(int64(listing.Metadata.Expiry), 0).Before(time.Now()) {
		return errors.New("Listing expiration must be in the future")
	}
	if listing.Item.Title == "" {
		return errors.New("Listing must have titles")
	}
	if listing.Item.Price.CurrencyCode == "" {
		return errors.New("Listing price currency code must not be nil")
	}
	if len(listing.Item.Title) > TitleMaxCharacters {
		return fmt.Errorf("Title is longer than the max of %d characters", TitleMaxCharacters)
	}
	if len(listing.Item.Description) > DescriptionMaxCharacters {
		return fmt.Errorf("Description is longer than the max of %d characters", DescriptionMaxCharacters)
	}
	if len(listing.Item.Tags) > MaxTags {
		return fmt.Errorf("Number of tags exceeds the max of %d", MaxTags)
	}
	for _, tag := range listing.Item.Tags {
		if tag == "" {
			return errors.New("Tags must not be nil")
		}
		if len(tag) > TagMaxCharacters {
			return fmt.Errorf("Tags must be less than max of %d", TagMaxCharacters)
		}
	}
	for _, img := range listing.Item.Images {
		_, err := mh.FromB58String(img.Hash)
		if err != nil {
			return errors.New("Image hashes must be a multihash")
		}
		if img.FileName == "" {
			return errors.New("Image file names must not be nil")
		}
	}
	for _, category := range listing.Item.Categories {
		if category == "" {
			return errors.New("Categories must not be nil")
		}
		if len(category) > CategoryMaxCharacters {
			return fmt.Errorf("Category length must be less than the max of %d", CategoryMaxCharacters)
		}
	}
	for _, option := range listing.Item.Options {
		if option.Title == "" {
			return errors.New("Options titles must not be nil")
		}
		if len(option.Variants) < 2 {
			return errors.New("Options must have more than one varients")
		}
		for _, variant := range option.Variants {
			if variant.Name == "" {
				return errors.New("Variant names must not be nil")
			}
			_, err := mh.FromB58String(variant.Image.Hash)
			if err != nil {
				return errors.New("Variant image hashes must be a multihash")
			}
			if variant.Image.FileName == "" {
				return errors.New("Variant image file names must not be nil")
			}
			if variant.PriceModifier.CurrencyCode == "" {
				return errors.New("Variant price modifier currency code must not be nil")
			}
		}
	}
	for _, shippingOption := range listing.ShippingOptions {
		if len(shippingOption.Regions) == 0 {
			return errors.New("Shipping options must specify at least one region")
		}
		for _, option := range shippingOption.Options {
			if option.Service == "" {
				return errors.New("Shipping option service name must not be nil")
			}
			if option.Price.CurrencyCode == "" {
				return errors.New("Shipping option price currency code must not be nil")
			}
			if option.EstimatedDelivery == "" {
				return errors.New("Shipping option estimated delivery must not be nil")
			}
		}
	}
	for _, shippingRule := range listing.ShippingRules {
		if int(shippingRule.RuleType) == 2 && listing.Item.Weight == 0 {
			return errors.New("Item weight must be specified when using FLAT_FEE_WEIGHT_RANGE shipping rule")
		}
		if len(shippingRule.Regions) == 0 {
			return errors.New("Shipping rules must specifiy at least one region")
		}
		if shippingRule.Price.CurrencyCode == "" {
			return errors.New("Shipping rules price currency code must not be nil")
		}
		if (int(shippingRule.RuleType) == 1 || int(shippingRule.RuleType) == 2) && shippingRule.MaxRange <= shippingRule.MinimumRange {
			return errors.New("Shipping rule max range cannot be less than or equal to the min range")
		}
		// TODO: For types 1 and 2 we should probably validate that the ranges used don't overlap
	}
	for _, tax := range listing.Tax {
		if tax.TaxType == "" {
			return errors.New("Tax type must be specified")
		}
		if len(tax.TaxRegions) == 0 {
			return errors.New("Tax must specifiy at least one region")
		}
		if tax.Percentage == 0 {
			return errors.New("No need to specify a tax if the rate is zero")
		}
	}
	for _, coupon := range listing.Coupons {
		if coupon.Title == "" {
			return errors.New("Coupon titles must not be nil")
		}
		if coupon.Discount.CurrencyCode == "" {
			return errors.New("Coupon price currency code must not be nil")
		}
		_, err := mh.FromB58String(coupon.Hash)
		if err != nil {
			return errors.New("Coupon hashes must be a multihash")
		}
	}
	for _, moderator := range listing.Moderators {
		_, err := mh.FromB58String(moderator)
		if err != nil {
			return errors.New("Moderator IDs must be a multihash")
		}
	}
	return nil
}
