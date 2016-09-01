package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/btcec"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	crypto "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const (
	ListingVersion           = 1
	TitleMaxCharacters       = 140
	ShortDescriptionLength   = 160
	DescriptionMaxCharacters = 50000
	MaxTags                  = 10
	WordMaxCharacters        = 40
	SentanceMaxCharacters    = 70
)

// Add our identity to the listing and sign it
func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.RicardianContract, error) {
	c := new(pb.RicardianContract)
	// Check the listing data is correct for continuing
	if err := validateListing(listing); err != nil {
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

	// Set cryoto currency
	listing.Metadata.AcceptedCryptoCurrency = n.Wallet.CurrencyCode()

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

	c.VendorListings = append(c.VendorListings, listing)
	c.Signatures = append(c.Signatures, s)
	return c, nil
}

// Sets the inventory for the listing in the database. Does some basic validation
// to make sure the inventory uses the correct variants.
func (n *OpenBazaarNode) SetListingInventory(listing *pb.Listing, inventory []*pb.Inventory) error {
	// Grap the current inventory for this listing
	currentInv, err := n.Datastore.Inventory().Get(listing.Slug)
	if err != nil {
		return err
	}
	// Delete from currentInv any variants that are carrying forward.
	// The remainder should be a map of variants that should be deleted.
	for _, i := range inventory {
		for k := range currentInv {
			if i.Item == k {
				delete(currentInv, k)
			}
		}
	}
	// Create a list of variants from the contract so we can check correct ordering
	var variants [][]string = make([][]string, len(listing.Item.Options))
	for i, option := range listing.Item.Options {
		var name []string
		for _, variant := range option.Variants {
			name = append(name, variant.Name)
		}
		variants[i] = name
	}
	for _, inv := range inventory {
		// format to remove leading and trailing path separator if one exists
		if string(inv.Item[0]) == "/" {
			inv.Item = inv.Item[1:]
		}
		if string(inv.Item[len(inv.Item)-1:len(inv.Item)]) == "/" {
			inv.Item = inv.Item[:len(inv.Item)-1]
		}
		names := strings.Split(inv.Item, "/")
		if names[0] != listing.Slug {
			return errors.New("Slug must be first item in inventory string")
		}
		if len(names) != len(variants)+1 {
			return errors.New("Incorrect number of variants in inventory string")
		}

		// Check ordering of inventory string matches options in listing item
	outer:
		for i, name := range names[1:] {
			for _, n := range variants[i] {
				if n == name {
					continue outer
				}
			}
			return fmt.Errorf("Inventory string in position %d is incorrect value", i+1)
		}
		// Put to database
		n.Datastore.Inventory().Put(inv.Item, int(inv.Count))
	}
	// Delete any variants that don't carry forward
	for k := range currentInv {
		err := n.Datastore.Inventory().Delete(k)
		if err != nil {
			return err
		}
	}
	return nil
}

// Update the index.json file in the listings directory
func (n *OpenBazaarNode) UpdateListingIndex(contract *pb.RicardianContract) error {
	type price struct {
		CurrencyCode string
		Amount       uint64
	}
	type listingData struct {
		Hash         string
		Slug         string
		Title        string
		Category     []string
		ContractType string
		Desc         string
		Thumbnail    string
		Price        price
	}
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	listingPath := path.Join(n.RepoPath, "root", "listings", contract.VendorListings[0].Slug, "listing.json")

	var index []listingData

	listingHash, err := ipfs.AddFile(n.Context, listingPath)
	if err != nil {
		return err
	}

	descLen := len(contract.VendorListings[0].Item.Description)
	if descLen > ShortDescriptionLength {
		descLen = ShortDescriptionLength
	}

	ld := listingData{
		Hash:         listingHash,
		Slug:         contract.VendorListings[0].Slug,
		Title:        contract.VendorListings[0].Item.Title,
		Category:     contract.VendorListings[0].Item.Categories,
		ContractType: contract.VendorListings[0].Metadata.ContractType.String(),
		Desc:         contract.VendorListings[0].Item.Description[:descLen],
		Thumbnail:    contract.VendorListings[0].Item.Images[0].Hash,
		Price:        price{contract.VendorListings[0].Item.Price.CurrencyCode, contract.VendorListings[0].Item.Price.Amount},
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

// Return the current number of listings
func (n *OpenBazaarNode) GetListingCount() int {
	type price struct {
		CurrencyCode string
		Amount       uint64
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

// Check to see we are selling the given listing. Used when validating an order.
// FIXME: this wont scale well. We will need a better way.
func (n *OpenBazaarNode) IsItemForSale(listing *pb.Listing) bool {
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		return false
	}
	type price struct {
		CurrencyCode string
		Amount       uint64
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
		log.Error(err)
		return false
	}

	var index []listingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		log.Error(err)
		return false
	}
	for _, l := range index {
		b, err := ipfs.Cat(n.Context, l.Hash)
		if err != nil {
			log.Error(err)
			return false
		}
		c := new(pb.RicardianContract)
		err = jsonpb.UnmarshalString(string(b), c)
		if err != nil {
			log.Error(err)
			return false
		}
		ser, err := proto.Marshal(c.VendorListings[0])
		if err != nil {
			log.Error(err)
			return false
		}
		if hex.EncodeToString(ser) == hex.EncodeToString(serializedListing) {
			return true
		}
	}
	return false
}

// Moves images from one directory to another.
// This is used when a user changes a slug and we need to copy images into
// the new listing directory.
func (n *OpenBazaarNode) TransferImages(fromSlug, toSlug string) error {
	fromPath := path.Join(n.RepoPath, "root", "listings", fromSlug)
	toPath := path.Join(n.RepoPath, "root", "listings", toSlug)

	directory, err := os.Open(fromPath)
	if err != nil {
		return err
	}
	defer directory.Close()
	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		sourcefilepointer := path.Join(fromPath, obj.Name())
		destinationfilepointer := path.Join(toPath, obj.Name())

		sourcefile, err := os.Open(sourcefilepointer)
		if err != nil {
			return err
		}

		defer sourcefile.Close()

		destfile, err := os.Create(destinationfilepointer)
		if err != nil {
			return err
		}

		defer destfile.Close()

		_, err = io.Copy(destfile, sourcefile)
		if err == nil {
			return err
		}
	}
	return nil
}

// Deletes the listing directory, removes the listing from the index, and deletes the inventory
func (n *OpenBazaarNode) DeleteListing(slug string) error {
	toDelete := path.Join(n.RepoPath, "root", "listings", slug)
	err := os.RemoveAll(toDelete)
	if err != nil {
		return err
	}
	type price struct {
		CurrencyCode string
		Amount       uint64
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

	var index []listingData
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
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

	// Check to see if the slug exists in the list. If so delete it.
	for i, d := range index {
		if d.Slug != slug {
			continue
		}

		if len(index) == 1 {
			index = []listingData{}
			break
		}
		index = append(index[:i], index[i+1:]...)
	}

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

	// Delete inventory for listing
	err = n.Datastore.Inventory().DeleteAll(slug)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) GetListingFromHash(hash string) (*pb.RicardianContract, []*pb.Inventory, error) {
	var contract *pb.RicardianContract
	type price struct {
		CurrencyCode string
		Amount       uint64
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
		return contract, nil, err
	}

	var index []listingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return contract, nil, err
	}
	var slug string
	for _, data := range index {
		if data.Hash == hash {
			slug = data.Slug
		}
	}
	if slug == "" {
		return contract, nil, errors.New("Listing does not exist")
	}
	return n.GetListingFromSlug(slug)
}

func (n *OpenBazaarNode) GetListingFromSlug(slug string) (*pb.RicardianContract, []*pb.Inventory, error) {
	listingPath := path.Join(n.RepoPath, "root", "listings", slug, "listing.json")

	var invList []*pb.Inventory
	contract := new(pb.RicardianContract)
	// read existing file
	file, err := ioutil.ReadFile(listingPath)
	if err != nil {
		return nil, nil, err
	}
	err = jsonpb.UnmarshalString(string(file), contract)
	if err != nil {
		return nil, nil, err
	}
	inventory, err := n.Datastore.Inventory().Get(contract.VendorListings[0].Slug)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range inventory {
		inv := new(pb.Inventory)
		inv.Item = k
		inv.Count = uint64(v)
		invList = append(invList, inv)
	}
	return contract, invList, nil
}

// Performs a ton of checks to make sure the listing is formatted correct. We shouldn't allow
// listings to be saved or purchased if they aren't formatted correctly as it can lead to
// ambiguity when moderating a dispute.
func validateListing(listing *pb.Listing) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()
	if listing.Slug == "" {
		return errors.New("Slug must not be nil")
	}
	if len(listing.Slug) > SentanceMaxCharacters {
		return fmt.Errorf("Slug lenght exceeds max of %d", SentanceMaxCharacters)
	}
	if listing.Metadata == nil {
		return errors.New("Missing required field: Metadata")
	}
	if listing.Metadata.ListingType == pb.Listing_Metadata_NA || int(listing.Metadata.ListingType) > 2 {
		return errors.New("Invalid listing type")
	}
	if listing.Metadata.ContractType == pb.Listing_Metadata_UNKNOWN || int(listing.Metadata.ContractType) > 4 {
		return errors.New("Invalid item type")
	}
	if listing.Metadata.Expiry == nil {
		return errors.New("Missing required field: Expiry")
	}
	if time.Unix(listing.Metadata.Expiry.Seconds, 0).Before(time.Now()) {
		return errors.New("Listing expiration must be in the future")
	}
	if listing.Item.Price == nil {
		return errors.New("Listings must have a price")
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
		if len(tag) > WordMaxCharacters {
			return fmt.Errorf("Tags must be less than max of %d", WordMaxCharacters)
		}
	}
	if len(listing.Item.Images) == 0 {
		return errors.New("Listing must contain at least one image")
	}
	for _, img := range listing.Item.Images {
		_, err := mh.FromB58String(img.Hash)
		if err != nil {
			return errors.New("Image hashes must be a multihash")
		}
		if img.FileName == "" {
			return errors.New("Image file names must not be nil")
		}
		if len(img.FileName) > SentanceMaxCharacters {
			return fmt.Errorf("Image filename length must be less than the max of %d", SentanceMaxCharacters)
		}
	}
	for _, category := range listing.Item.Categories {
		if category == "" {
			return errors.New("Categories must not be nil")
		}
		if len(category) > WordMaxCharacters {
			return fmt.Errorf("Category length must be less than the max of %d", WordMaxCharacters)
		}
	}
	if len(listing.Item.ProcessingTime) > SentanceMaxCharacters {
		return fmt.Errorf("Processing time length must be less than the max of %d", SentanceMaxCharacters)
	}
	if len(listing.Item.Sku) > SentanceMaxCharacters {
		return fmt.Errorf("Sku length must be less than the max of %d", SentanceMaxCharacters)
	}
	if len(listing.Item.Condition) > SentanceMaxCharacters {
		return fmt.Errorf("Condition length must be less than the max of %d", SentanceMaxCharacters)
	}
	for _, option := range listing.Item.Options {
		if option.Name == "" {
			return errors.New("Options titles must not be nil")
		}
		if len(option.Variants) < 2 {
			return errors.New("Options must have more than one varients")
		}
		if len(option.Name) > WordMaxCharacters {
			return fmt.Errorf("Option title length must be less than the max of %d", WordMaxCharacters)
		}
		if len(option.Description) > SentanceMaxCharacters {
			return fmt.Errorf("Option description length must be less than the max of %d", SentanceMaxCharacters)
		}
		for _, variant := range option.Variants {
			if variant.Name == "" {
				return errors.New("Variant names must not be nil")
			}
			if len(variant.Name) > WordMaxCharacters {
				return fmt.Errorf("Variant name length must be less than the max of %d", WordMaxCharacters)
			}
			if variant.Image != nil {
				_, err := mh.FromB58String(variant.Image.Hash)
				if err != nil {
					return errors.New("Variant image hashes must be a multihash")
				}
				if len(variant.Image.FileName) > SentanceMaxCharacters {
					return fmt.Errorf("Variant image filename length must be less than the max of %d", SentanceMaxCharacters)
				}
				if variant.Image.FileName == "" {
					return errors.New("Variant image file names must not be nil")
				}
			}
			if variant.PriceModifier != nil {
				if variant.PriceModifier.CurrencyCode == "" {
					return errors.New("Variant price modifier currency code must not be nil")
				}
			}
		}
	}
	var shippingTitles []string
	for _, shippingOption := range listing.ShippingOptions {
		if len(shippingOption.Regions) == 0 {
			return errors.New("Shipping options must specify at least one region")
		}
		if shippingOption.Name == "" {
			return errors.New("Shipping option title name must not be nil")
		}
		if len(shippingOption.Name) > WordMaxCharacters {
			return fmt.Errorf("Shipping option service length must be less than the max of %d", WordMaxCharacters)
		}
		for _, t := range shippingTitles {
			if t == shippingOption.Name {
				return errors.New("Shipping option titles must be unique")
			}
		}
		if shippingOption.ShippingRules != nil {
			if len(shippingOption.ShippingRules.Rules) == 0 {
				return errors.New("At least on rule must be specified if ShippingRules is selected")
			}
			if shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_WEIGHT_RANGE && listing.Item.Grams == 0 {
				return errors.New("Item weight must be specified when using FLAT_FEE_WEIGHT_RANGE shipping rule")
			}
			if (shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_ADD || shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_SUBTRACT) && len(shippingOption.ShippingRules.Rules) > 1 {
				return errors.New("Selected shipping rule type can only have a maximum of one rule")
			}
			for _, rule := range shippingOption.ShippingRules.Rules {
				if rule.Price == nil {
					return errors.New("Shipping rules must have a price")
				}
				if rule.Price.CurrencyCode == "" {
					return errors.New("Shipping rules price currency code must not be nil")
				}
				if (shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_QUANTITY_RANGE || shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_WEIGHT_RANGE) && rule.MaxRange <= rule.MinRange {
					return errors.New("Shipping rule max range cannot be less than or equal to the min range")
				}
			}
		}
		// TODO: For types 1 and 2 we should probably validate that the ranges used don't overlap
		shippingTitles = append(shippingTitles, shippingOption.Name)
		var serviceTitles []string
		for _, option := range shippingOption.Services {

			if option.Name == "" {
				return errors.New("Shipping option service name must not be nil")
			}
			if len(option.Name) > WordMaxCharacters {
				return fmt.Errorf("Shipping option service length must be less than the max of %d", WordMaxCharacters)
			}
			for _, t := range serviceTitles {
				if t == option.Name {
					return errors.New("Shipping option services names must be unique")
				}
			}
			serviceTitles = append(serviceTitles, option.Name)
			if option.Price.CurrencyCode == "" {
				return errors.New("Shipping option price currency code must not be nil")
			}
			if option.EstimatedDelivery == "" {
				return errors.New("Shipping option estimated delivery must not be nil")
			}
			if len(option.EstimatedDelivery) > SentanceMaxCharacters {
				return fmt.Errorf("Shipping option estimated delivery length must be less than the max of %d", SentanceMaxCharacters)
			}
		}
	}
	for _, tax := range listing.Taxes {
		if tax.TaxType == "" {
			return errors.New("Tax type must be specified")
		}
		if len(tax.TaxType) > WordMaxCharacters {
			return fmt.Errorf("Tax type length must be less than the max of %d", WordMaxCharacters)
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
		if len(coupon.Title) > SentanceMaxCharacters {
			return fmt.Errorf("Coupon title length must be less than the max of %d", SentanceMaxCharacters)
		}

		if coupon.PriceDiscount != nil {
			if coupon.PriceDiscount.CurrencyCode == "" {
				return errors.New("Price discount coupon currency code must not be nil")
			}
			if coupon.PercentDiscount > 0 {
				return errors.New("Only one type of coupon discount can be selected")
			}
		} else if coupon.PercentDiscount <= 0 {
			return errors.New("The coupon discount must be selected")
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

func verifySignaturesOnListing(contract *pb.RicardianContract) error {
	for n, listing := range contract.VendorListings {
		guidPubkeyBytes := listing.VendorID.Pubkeys.Guid
		bitcoinPubkeyBytes := listing.VendorID.Pubkeys.Bitcoin
		guid := listing.VendorID.Guid
		ser, err := proto.Marshal(listing)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(ser)
		guidPubkey, err := crypto.UnmarshalPublicKey(guidPubkeyBytes)
		if err != nil {
			return err
		}
		bitcoinPubkey, err := btcec.ParsePubKey(bitcoinPubkeyBytes, btcec.S256())
		if err != nil {
			return err
		}
		var guidSig []byte
		var bitcoinSig *btcec.Signature
		sig := contract.Signatures[n]
		if sig.Section != pb.Signatures_LISTING {
			return errors.New("Contract does not contain listing signature")
		}
		guidSig = sig.Guid
		bitcoinSig, err = btcec.ParseSignature(sig.Bitcoin, btcec.S256())
		if err != nil {
			return err
		}
		valid, err := guidPubkey.Verify(ser, guidSig)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("Vendor's guid signature on contact failed to verify")
		}
		checkKeyHash, err := guidPubkey.Hash()
		if err != nil {
			return err
		}
		guidMH, err := mh.FromB58String(guid)
		if err != nil {
			return err
		}
		for i, b := range []byte(guidMH) {
			if b != checkKeyHash[i] {
				return errors.New("Public key in listing does not match reported vendor ID")
			}
		}
		valid = bitcoinSig.Verify(hash[:], bitcoinPubkey)
		if !valid {
			return errors.New("Vendor's bitcoin signature on contact failed to verify")
		}
	}
	return nil
}
