package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"github.com/kennygrant/sanitize"
)

const (
	ListingVersion           = 1
	TitleMaxCharacters       = 140
	ShortDescriptionLength   = 160
	DescriptionMaxCharacters = 50000
	MaxTags                  = 10
	MaxCategories            = 10
	MaxListItems             = 30
	FilenameMaxCharacters    = 255
	WordMaxCharacters        = 40
	SentenceMaxCharacters    = 70
	PolicyMaxCharacters      = 10000
	MaxCountryCodes          = 255
)

type price struct {
	CurrencyCode string `json:"currencyCode"`
	Amount       uint64 `json:"amount"`
}
type thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}
type listingData struct {
	Hash         string    `json:"hash"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Category     []string  `json:"category"`
	ContractType string    `json:"contractType"`
	Description  string    `json:"description"`
	Thumbnail    thumbnail `json:"thumbnail"`
	Price        price     `json:"price"`
	ShipsTo      []string  `json:"shipsTo"`
	FreeShipping []string  `json:"freeShipping"`
}

// Add our identity to the listing and sign it
func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.RicardianContract, error) {
	slugFromTitle := func(title string) string {
		l := TitleMaxCharacters
		if len(title) < TitleMaxCharacters {
			l = len(title)
		}
		return url.QueryEscape(sanitize.Path(strings.ToLower(title[:l])))
	}

	c := new(pb.RicardianContract)
	// If the slug is empty, create one from the title
	if listing.Slug == "" {
		counter := 1
		slugBase := slugFromTitle(listing.Item.Title)
		slugToTry := slugBase
		for {
			_, _, err := n.GetListingFromSlug(slugToTry)
			if err != nil {
				listing.Slug = slugToTry
				break
			}
			slugToTry = slugBase + strconv.Itoa(counter)
			counter++
		}
	}

	// Check the listing data is correct for continuing
	if err := validateListing(listing); err != nil {
		return c, err
	}

	// Set listing version
	listing.Metadata.Version = ListingVersion

	// Add the vendor ID to the listing
	id := new(pb.ID)
	id.Guid = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return c, err
	}
	profile, err := n.GetProfile()
	if err == nil {
		id.BlockchainID = profile.Handle
	}
	p := new(pb.ID_Pubkeys)
	p.Guid = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return c, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	listing.VendorID = id

	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return c, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.Guid))
	id.BitcoinSig = sig.Serialize()

	// Set crypto currency
	listing.Metadata.AcceptedCurrency = n.Wallet.CurrencyCode()

	// Sign listing
	s := new(pb.Signature)
	s.Section = pb.Signature_LISTING
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		return c, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedListing)
	if err != nil {
		return c, err
	}
	s.SignatureBytes = guidSig
	c.VendorListings = append(c.VendorListings, listing)
	c.Signatures = append(c.Signatures, s)
	return c, nil
}

/* Sets the inventory for the listing in the database. Does some basic validation
   to make sure the inventory uses the correct variants. */
func (n *OpenBazaarNode) SetListingInventory(listing *pb.Listing, inventory []*pb.Inventory) error {
	// Format to remove leading and trailing path separator if one exists
	for _, inv := range inventory {
		if string(inv.Item[0]) == "/" {
			inv.Item = inv.Item[1:]
		}
		if string(inv.Item[len(inv.Item)-1:len(inv.Item)]) == "/" {
			inv.Item = inv.Item[:len(inv.Item)-1]
		}
		s := strings.Split(inv.Item, "/")
		if s[0] != listing.Slug {
			inv.Item = path.Join(listing.Slug, inv.Item)
		}
	}
	// Grab the current inventory for this listing
	currentInv, err := n.Datastore.Inventory().Get(listing.Slug)
	if err != nil {
		return err
	}
	/* Delete from currentInv any variants that are carrying forward.
	   The remainder should be a map of variants that should be deleted. */
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
	// Delete any variants that do not carry forward
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
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	listingPath := path.Join(n.RepoPath, "root", "listings", contract.VendorListings[0].Slug+".json")

	var index []listingData

	listingHash, err := ipfs.GetHash(n.Context, listingPath)
	if err != nil {
		return err
	}

	descriptionLength := len(contract.VendorListings[0].Item.Description)
	if descriptionLength > ShortDescriptionLength {
		descriptionLength = ShortDescriptionLength
	}

	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	shipsTo := []string{}
	freeShipping := []string{}
	for _, shippingOption := range contract.VendorListings[0].ShippingOptions {
		for _, region := range shippingOption.Regions {
			if !contains(shipsTo, region.String()) {
				shipsTo = append(shipsTo, region.String())
			}
			for _, service := range shippingOption.Services {
				if service.Price == 0 && !contains(freeShipping, region.String()) {
					freeShipping = append(freeShipping, region.String())
				}
			}
		}
	}

	ld := listingData{
		Hash:         listingHash,
		Slug:         contract.VendorListings[0].Slug,
		Title:        contract.VendorListings[0].Item.Title,
		Category:     contract.VendorListings[0].Item.Categories,
		ContractType: contract.VendorListings[0].Metadata.ContractType.String(),
		Description:  contract.VendorListings[0].Item.Description[:descriptionLength],
		Thumbnail:    thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small, contract.VendorListings[0].Item.Images[0].Medium},
		Price:        price{contract.VendorListings[0].Metadata.PricingCurrency, contract.VendorListings[0].Item.Price},
		ShipsTo:      shipsTo,
		FreeShipping: freeShipping,
	}

	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// Read existing file
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

	// Write it back to file
	f, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer f.Close()

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

// Update the hashes in the index.json file
func (n *OpenBazaarNode) UpdateIndexHashes(hashes map[string]string) error {
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")

	var index []listingData

	_, ferr := os.Stat(indexPath)
	if os.IsNotExist(ferr) {
		return ferr
	}
	// Read existing file
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, &index)
	if err != nil {
		return err
	}

	// Update hashes
	for _, d := range index {
		hash, ok := hashes[d.Slug]
		if ok {
			d.Hash = hash
		}
	}

	// Write it back to file
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
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")

	// Read existing file
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
// FIXME: This wont scale well. We will need to store the hash of active listings in a db to do an indexed search.
func (n *OpenBazaarNode) IsItemForSale(listing *pb.Listing) bool {
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		log.Error(err)
		return false
	}
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")

	// Read existing file
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
		if bytes.Equal(ser, serializedListing) {
			return true
		}
	}
	return false
}

// Deletes the listing directory, removes the listing from the index, and deletes the inventory
func (n *OpenBazaarNode) DeleteListing(slug string) error {
	toDelete := path.Join(n.RepoPath, "root", "listings", slug+".json")
	err := os.Remove(toDelete)
	if err != nil {
		return err
	}
	var index []listingData
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) { // FIXME: What if there is an error other than NotExist?
		// Read existing file
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

	// Write the index back to file
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

	return n.updateProfileCounts()
}

func (n *OpenBazaarNode) GetListings() ([]byte, error) {
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	file, err := ioutil.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return []byte("[]"), nil
	} else if err != nil {
		return nil, err
	}

	// Unmarshal the index to check if file contains valid json
	var index []listingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return nil, err
	}

	// Return bytes read from file
	return file, nil
}

func (n *OpenBazaarNode) GetListingFromHash(hash string) (*pb.RicardianContract, []*pb.Inventory, error) {
	// Read index.json
	indexPath := path.Join(n.RepoPath, "root", "listings", "index.json")
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return nil, nil, err
	}

	// Unmarshal the index
	var index []listingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return nil, nil, err
	}

	// Extract slug that matches hash
	var slug string
	for _, data := range index {
		if data.Hash == hash {
			slug = data.Slug
			break
		}
	}

	if slug == "" {
		return nil, nil, errors.New("Listing does not exist")
	}
	return n.GetListingFromSlug(slug)
}

func (n *OpenBazaarNode) GetListingFromSlug(slug string) (*pb.RicardianContract, []*pb.Inventory, error) {
	// Read listing file
	listingPath := path.Join(n.RepoPath, "root", "listings", slug+".json")
	file, err := ioutil.ReadFile(listingPath)
	if err != nil {
		return nil, nil, err
	}

	// Unmarshal listing
	contract := new(pb.RicardianContract)
	err = jsonpb.UnmarshalString(string(file), contract)
	if err != nil {
		return nil, nil, err
	}

	// Get the listing inventory
	inventory, err := n.Datastore.Inventory().Get(contract.VendorListings[0].Slug) // FIXME: Can this be simplified to Get(slug)?
	if err != nil {
		return nil, nil, err
	}

	// Build the inventory list
	var invList []*pb.Inventory
	for k, v := range inventory {
		inv := new(pb.Inventory)
		inv.Item = k
		inv.Count = uint64(v)
		invList = append(invList, inv)
	}
	return contract, invList, nil
}

/* Performs a ton of checks to make sure the listing is formatted correctly. We should not allow
   invalid listings to be saved or purchased as it can lead to ambiguity when moderating a dispute
   or possible attacks. This function needs to be maintained in conjunction with contracts.proto */
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

	// Slug
	if listing.Slug == "" {
		return errors.New("Slug must not be empty")
	}
	if len(listing.Slug) > SentenceMaxCharacters {
		return fmt.Errorf("Slug is longer than the max of %d", SentenceMaxCharacters)
	}
	if strings.Contains(listing.Slug, " ") {
		return errors.New("Slugs cannot contain spaces")
	}

	// Metadata
	if listing.Metadata == nil {
		return errors.New("Missing required field: Metadata")
	}
	if listing.Metadata.ContractType > pb.Listing_Metadata_SERVICE {
		return errors.New("Invalid contract type")
	}
	if listing.Metadata.Format > pb.Listing_Metadata_AUCTION {
		return errors.New("Invalid listing format")
	}
	if listing.Metadata.Expiry == nil {
		return errors.New("Missing required field: Expiry")
	}
	if time.Unix(listing.Metadata.Expiry.Seconds, 0).Before(time.Now()) {
		return errors.New("Listing expiration must be in the future")
	}
	if listing.Metadata.PricingCurrency == "" {
		return errors.New("Listing pricing currency code must not be empty")
	}

	// Item
	if listing.Item.Title == "" {
		return errors.New("Listing must have a title")
	}
	if len(listing.Item.Title) > TitleMaxCharacters {
		return fmt.Errorf("Title is longer than the max of %d characters", TitleMaxCharacters)
	}
	if len(listing.Item.Description) > DescriptionMaxCharacters {
		return fmt.Errorf("Description is longer than the max of %d characters", DescriptionMaxCharacters)
	}
	if len(listing.Item.ProcessingTime) > SentenceMaxCharacters {
		return fmt.Errorf("Processing time length must be less than the max of %d", SentenceMaxCharacters)
	}
	if len(listing.Item.Tags) > MaxTags {
		return fmt.Errorf("Number of tags exceeds the max of %d", MaxTags)
	}
	for _, tag := range listing.Item.Tags {
		if tag == "" {
			return errors.New("Tags must not be empty")
		}
		if len(tag) > WordMaxCharacters {
			return fmt.Errorf("Tags must be less than max of %d", WordMaxCharacters)
		}
	}
	if len(listing.Item.Images) == 0 {
		return errors.New("Listing must contain at least one image")
	}
	if len(listing.Item.Images) > MaxListItems {
		return fmt.Errorf("Number of listing images is greater than the max of %d", MaxListItems)
	}
	for _, img := range listing.Item.Images {
		_, err := mh.FromB58String(img.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be multihashes")
		}
		_, err = mh.FromB58String(img.Small)
		if err != nil {
			return errors.New("Small image hashes must be multihashes")
		}
		_, err = mh.FromB58String(img.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be multihashes")
		}
		_, err = mh.FromB58String(img.Large)
		if err != nil {
			return errors.New("Large image hashes must be multihashes")
		}
		_, err = mh.FromB58String(img.Original)
		if err != nil {
			return errors.New("Original image hashes must be multihashes")
		}
		if img.Filename == "" {
			return errors.New("Image file names must not be nil")
		}
		if len(img.Filename) > FilenameMaxCharacters {
			return fmt.Errorf("Image filename length must be less than the max of %d", FilenameMaxCharacters)
		}
	}
	if len(listing.Item.Categories) > MaxCategories {
		return fmt.Errorf("Number of categories must be less than max of %d", MaxCategories)
	}
	for _, category := range listing.Item.Categories {
		if category == "" {
			return errors.New("Categories must not be nil")
		}
		if len(category) > WordMaxCharacters {
			return fmt.Errorf("Category length must be less than the max of %d", WordMaxCharacters)
		}
	}
	if len(listing.Item.Sku) > WordMaxCharacters {
		return fmt.Errorf("Sku length must be less than the max of %d", WordMaxCharacters)
	}
	if len(listing.Item.Condition) > SentenceMaxCharacters {
		return fmt.Errorf("Condition length must be less than the max of %d", SentenceMaxCharacters)
	}
	if len(listing.Item.Options) > MaxListItems {
		return fmt.Errorf("Number of options is greater than the max of %d", MaxListItems)
	}
	for _, option := range listing.Item.Options {
		if option.Name == "" {
			return errors.New("Options titles must not be empty")
		}
		if len(option.Variants) < 2 {
			return errors.New("Options must have more than one variants")
		}
		if len(option.Name) > WordMaxCharacters {
			return fmt.Errorf("Option title length must be less than the max of %d", WordMaxCharacters)
		}
		if len(option.Description) > SentenceMaxCharacters {
			return fmt.Errorf("Option description length must be less than the max of %d", SentenceMaxCharacters)
		}
		if len(option.Variants) > MaxListItems {
			return fmt.Errorf("Number of variants is greater than the max of %d", MaxListItems)
		}
		for _, variant := range option.Variants {
			if variant.Name == "" {
				return errors.New("Variant names must not be empty")
			}
			if len(variant.Name) > WordMaxCharacters {
				return fmt.Errorf("Variant name length must be less than the max of %d", WordMaxCharacters)
			}
			if variant.Image != nil {
				_, err := mh.FromB58String(variant.Image.Tiny)
				if err != nil {
					return errors.New("Tiny image hashes must be multihashes")
				}
				_, err = mh.FromB58String(variant.Image.Small)
				if err != nil {
					return errors.New("Small image hashes must be multihashes")
				}
				_, err = mh.FromB58String(variant.Image.Medium)
				if err != nil {
					return errors.New("Medium image hashes must be multihashes")
				}
				_, err = mh.FromB58String(variant.Image.Large)
				if err != nil {
					return errors.New("Large image hashes must be multihashes")
				}
				_, err = mh.FromB58String(variant.Image.Original)
				if err != nil {
					return errors.New("Original image hashes must be multihashes")
				}
				if variant.Image.Filename == "" {
					return errors.New("Variant image file names must not be empty")
				}
				if len(variant.Image.Filename) > SentenceMaxCharacters {
					return fmt.Errorf("Variant image filename length must be less than the max of %d", SentenceMaxCharacters)
				}
			}
		}
	}

	// ShippingOptions
	if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD && len(listing.ShippingOptions) == 0 {
		return errors.New("Must be at least one shipping option for a physical good")
	}
	if len(listing.ShippingOptions) > MaxListItems {
		return fmt.Errorf("Number of shipping options is greater than the max of %d", MaxListItems)
	}
	var shippingTitles []string
	for _, shippingOption := range listing.ShippingOptions {
		if shippingOption.Name == "" {
			return errors.New("Shipping option title name must not be empty")
		}
		if len(shippingOption.Name) > WordMaxCharacters {
			return fmt.Errorf("Shipping option service length must be less than the max of %d", WordMaxCharacters)
		}
		for _, t := range shippingTitles {
			if t == shippingOption.Name {
				return errors.New("Shipping option titles must be unique")
			}
		}
		shippingTitles = append(shippingTitles, shippingOption.Name)
		if shippingOption.Type > pb.Listing_ShippingOption_FIXED_PRICE {
			return errors.New("Unkown shipping option type")
		}
		if len(shippingOption.Regions) == 0 {
			return errors.New("Shipping options must specify at least one region")
		}
		if len(shippingOption.Regions) > MaxCountryCodes {
			return fmt.Errorf("Number of shipping regions is greater than the max of %d", MaxCountryCodes)
		}
		if shippingOption.ShippingRules != nil {
			if len(shippingOption.ShippingRules.Rules) == 0 {
				return errors.New("At least on rule must be specified if ShippingRules is selected")
			}
			if len(shippingOption.ShippingRules.Rules) > MaxListItems {
				return fmt.Errorf("Number of shipping rules is greater than the max of %d", MaxListItems)
			}
			if shippingOption.ShippingRules.RuleType > pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_SUBTRACT {
				return errors.New("Unknown shipping rule")
			}
			if shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_WEIGHT_RANGE && listing.Item.Grams == 0 {
				return errors.New("Item weight must be specified when using FLAT_FEE_WEIGHT_RANGE shipping rule")
			}
			if (shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_ADD || shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_SUBTRACT) && len(shippingOption.ShippingRules.Rules) > 1 {
				return errors.New("Selected shipping rule type can only have a maximum of one rule")
			}
			for _, rule := range shippingOption.ShippingRules.Rules {
				if (shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_QUANTITY_RANGE || shippingOption.ShippingRules.RuleType == pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_WEIGHT_RANGE) && rule.MaxRange <= rule.MinRange {
					return errors.New("Shipping rule max range cannot be less than or equal to the min range")
				}
			}
		}
		if len(shippingOption.Services) == 0 && shippingOption.Type != pb.Listing_ShippingOption_LOCAL_PICKUP {
			return errors.New("At least one service must be specified for a shipping option when not local pickup")
		}
		if len(shippingOption.Services) > MaxListItems {
			return fmt.Errorf("Number of shipping services is greater than the max of %d", MaxListItems)
		}
		var serviceTitles []string
		for _, option := range shippingOption.Services {
			if option.Name == "" {
				return errors.New("Shipping option service name must not be empty")
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
			if option.EstimatedDelivery == "" {
				return errors.New("Shipping option estimated delivery must not be empty")
			}
			if len(option.EstimatedDelivery) > SentenceMaxCharacters {
				return fmt.Errorf("Shipping option estimated delivery length must be less than the max of %d", SentenceMaxCharacters)
			}
		}
	}

	// Taxes
	if len(listing.Taxes) > MaxListItems {
		return fmt.Errorf("Number of taxes is greater than the max of %d", MaxListItems)
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
		if len(tax.TaxRegions) > MaxCountryCodes {
			return fmt.Errorf("Number of tax regions is greater than the max of %d", MaxCountryCodes)
		}
		if tax.Percentage == 0 || tax.Percentage > 100 {
			return errors.New("Tax percentage must be between 0 and 100")
		}
	}

	// Coupons
	if len(listing.Coupons) > MaxListItems {
		return fmt.Errorf("Number of coupons is greater than the max of %d", MaxListItems)
	}
	for _, coupon := range listing.Coupons {
		if coupon.Title == "" {
			return errors.New("Coupon titles must not be empty")
		}
		if len(coupon.Title) > SentenceMaxCharacters {
			return fmt.Errorf("Coupon title length must be less than the max of %d", SentenceMaxCharacters)
		}
		_, err := mh.FromB58String(coupon.Hash)
		if err != nil {
			return errors.New("Coupon hashes must be multihashes")
		}
		if coupon.GetPercentDiscount() > 100 {
			return errors.New("Percent discount cannot be over 100 percent")
		}
		if coupon.GetPriceDiscount() > listing.Item.Price {
			return errors.New("Price discount cannot be greater than the item price")
		}
		if coupon.GetPercentDiscount() == 0 && coupon.GetPriceDiscount() == 0 {
			return errors.New("Coupons must have at least one positive discount value")
		}
	}

	// Moderators
	if len(listing.Moderators) > MaxListItems {
		return fmt.Errorf("Number of moderators is greater than the max of %d", MaxListItems)
	}
	for _, moderator := range listing.Moderators {
		_, err := mh.FromB58String(moderator)
		if err != nil {
			return errors.New("Moderator IDs must be multihashes")
		}
	}

	// TermsAndConditions
	if len(listing.TermsAndConditions) > PolicyMaxCharacters {
		return fmt.Errorf("Terms and conditions length must be less than the max of %d", PolicyMaxCharacters)
	}

	// RefundPolicy
	if len(listing.RefundPolicy) > PolicyMaxCharacters {
		return fmt.Errorf("Refun policy length must be less than the max of %d", PolicyMaxCharacters)
	}

	return nil
}

func verifySignaturesOnListing(contract *pb.RicardianContract) error {
	for _, listing := range contract.VendorListings {
		// Verify identity signature on listing
		if err := verifyMessageSignature(
			listing,
			listing.VendorID.Pubkeys.Guid,
			contract.Signatures,
			pb.Signature_LISTING,
			listing.VendorID.Guid,
		); err != nil {
			switch err.(type) {
			case noSigError:
				return errors.New("Contract does not contain listing signature")
			case invalidSigError:
				return errors.New("Buyer's guid signature on contact failed to verify")
			case matchKeyError:
				return errors.New("Public key in order does not match reported buyer ID")
			default:
				return err
			}
		}

		// Verify the bitcoin signature in the ID
		if err := verifyBitcoinSignature(
			listing.VendorID.Pubkeys.Bitcoin,
			listing.VendorID.BitcoinSig,
			listing.VendorID.Guid,
		); err != nil {
			switch err.(type) {
			case invalidSigError:
				return errors.New("Vendor's bitcoin signature on GUID failed to verify")
			default:
				return err
			}
		}
	}
	return nil
}
