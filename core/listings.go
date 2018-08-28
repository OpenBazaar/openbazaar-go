package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/kennygrant/sanitize"
	"github.com/microcosm-cc/bluemonday"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

const (
	// ListingVersion - current listing version
	ListingVersion = 4
	// TitleMaxCharacters - max size for title
	TitleMaxCharacters = 140
	// ShortDescriptionLength - min length for description
	ShortDescriptionLength = 160
	// DescriptionMaxCharacters - max length for description
	DescriptionMaxCharacters = 50000
	// MaxTags - max permitted tags
	MaxTags = 10
	// MaxCategories - max permitted categories
	MaxCategories = 10
	// MaxListItems - max items in a listing
	MaxListItems = 30
	// FilenameMaxCharacters - max filename size
	FilenameMaxCharacters = 255
	// CodeMaxCharacters - max chars for a code
	CodeMaxCharacters = 20
	// WordMaxCharacters - max chars for word
	WordMaxCharacters = 40
	// SentenceMaxCharacters - max chars for sentence
	SentenceMaxCharacters = 70
	// CouponTitleMaxCharacters - max length of a coupon title
	CouponTitleMaxCharacters = 70
	// PolicyMaxCharacters - max length for policy
	PolicyMaxCharacters = 10000
	// AboutMaxCharacters - max length for about
	AboutMaxCharacters = 10000
	// URLMaxCharacters - max length for URL
	URLMaxCharacters = 2000
	// MaxCountryCodes - max country codes
	MaxCountryCodes = 255
	// EscrowTimeout - escrow timeout in hours
	EscrowTimeout = 1080
	// SlugBuffer - buffer size for slug
	SlugBuffer = 5
	// PriceModifierMin - min price modifier
	PriceModifierMin = -99.99
	// PriceModifierMax = max price modifier
	PriceModifierMax = 1000.00

	// DefaultCoinDivisibility - decimals for price
	DefaultCoinDivisibility uint32 = 1e8
)

type price struct {
	CurrencyCode string  `json:"currencyCode"`
	Amount       uint64  `json:"amount"`
	Modifier     float32 `json:"modifier"`
}
type thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

// ListingData - represent a listing
type ListingData struct {
	Hash               string    `json:"hash"`
	Slug               string    `json:"slug"`
	Title              string    `json:"title"`
	Categories         []string  `json:"categories"`
	NSFW               bool      `json:"nsfw"`
	ContractType       string    `json:"contractType"`
	Description        string    `json:"description"`
	Thumbnail          thumbnail `json:"thumbnail"`
	Price              price     `json:"price"`
	ShipsTo            []string  `json:"shipsTo"`
	FreeShipping       []string  `json:"freeShipping"`
	Language           string    `json:"language"`
	AverageRating      float32   `json:"averageRating"`
	RatingCount        uint32    `json:"ratingCount"`
	ModeratorIDs       []string  `json:"moderators"`
	AcceptedCurrencies []string  `json:"acceptedCurrencies"`
	CoinType           string    `json:"coinType"`
}

// GenerateSlug - slugify the title of the listing
func (n *OpenBazaarNode) GenerateSlug(title string) (string, error) {
	title = strings.Replace(title, "/", "", -1)
	slugFromTitle := func(title string) string {
		l := SentenceMaxCharacters - SlugBuffer
		if len(title) < SentenceMaxCharacters-SlugBuffer {
			l = len(title)
		}
		return url.QueryEscape(sanitize.Path(strings.ToLower(title[:l])))
	}
	counter := 1
	slugBase := slugFromTitle(title)
	slugToTry := slugBase
	for {
		_, err := n.GetListingFromSlug(slugToTry)
		if os.IsNotExist(err) {
			return slugToTry, nil
		} else if err != nil {
			return "", err
		}
		slugToTry = slugBase + strconv.Itoa(counter)
		counter++
	}
}

// SignListing Add our identity to the listing and sign it
func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.SignedListing, error) {
	// Set inventory to the default as it's not part of the contract
	for _, s := range listing.Item.Skus {
		s.Quantity = 0
	}

	sl := new(pb.SignedListing)

	// Temporary hack to work around test env shortcomings
	if n.TestNetworkEnabled() || n.RegressionNetworkEnabled() {
		if listing.Metadata.EscrowTimeoutHours == 0 {
			listing.Metadata.EscrowTimeoutHours = 1
		}
	} else {
		listing.Metadata.EscrowTimeoutHours = EscrowTimeout
	}

	// Set crypto currency
	listing.Metadata.AcceptedCurrencies = []string{NormalizeCurrencyCode(n.Wallet.CurrencyCode())}

	// Sanitize a few critical fields
	if listing.Item == nil {
		return sl, errors.New("No item in listing")
	}
	sanitizer := bluemonday.UGCPolicy()
	for _, opt := range listing.Item.Options {
		opt.Name = sanitizer.Sanitize(opt.Name)
		for _, v := range opt.Variants {
			v.Name = sanitizer.Sanitize(v.Name)
		}
	}
	for _, so := range listing.ShippingOptions {
		so.Name = sanitizer.Sanitize(so.Name)
		for _, serv := range so.Services {
			serv.Name = sanitizer.Sanitize(serv.Name)
		}
	}

	// Check the listing data is correct for continuing
	testingEnabled := n.TestNetworkEnabled() || n.RegressionNetworkEnabled()
	if err := validateListing(listing, testingEnabled); err != nil {
		return sl, err
	}

	// Set listing version
	listing.Metadata.Version = ListingVersion

	// Add the vendor ID to the listing
	id := new(pb.ID)
	id.PeerID = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return sl, err
	}
	profile, err := n.GetProfile()
	if err == nil {
		id.Handle = profile.Handle
	}
	p := new(pb.ID_Pubkeys)
	p.Identity = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return sl, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	listing.VendorID = id

	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return sl, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.PeerID))
	id.BitcoinSig = sig.Serialize()

	// Update coupon db
	n.Datastore.Coupons().Delete(listing.Slug)
	var couponsToStore []repo.Coupon
	for i, coupon := range listing.Coupons {
		hash := coupon.GetHash()
		code := coupon.GetDiscountCode()
		_, err := mh.FromB58String(hash)
		if err != nil {
			couponMH, err := EncodeMultihash([]byte(code))
			if err != nil {
				return sl, err
			}

			listing.Coupons[i].Code = &pb.Listing_Coupon_Hash{Hash: couponMH.B58String()}
			hash = couponMH.B58String()
		}
		c := repo.Coupon{Slug: listing.Slug, Code: code, Hash: hash}
		couponsToStore = append(couponsToStore, c)
	}
	err = n.Datastore.Coupons().Put(couponsToStore)
	if err != nil {
		return sl, err
	}

	// Sign listing
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		return sl, err
	}
	idSig, err := n.IpfsNode.PrivateKey.Sign(serializedListing)
	if err != nil {
		return sl, err
	}
	sl.Listing = listing
	sl.Signature = idSig
	return sl, nil
}

/*SetListingInventory Sets the inventory for the listing in the database. Does some basic validation
  to make sure the inventory uses the correct variants. */
func (n *OpenBazaarNode) SetListingInventory(listing *pb.Listing) error {
	err := validateListingSkus(listing)
	if err != nil {
		return err
	}

	// Grab current inventory
	currentInv, err := n.Datastore.Inventory().Get(listing.Slug)
	if err != nil {
		return err
	}
	// Update inventory
	for i, s := range listing.Item.Skus {
		err = n.Datastore.Inventory().Put(listing.Slug, i, s.Quantity)
		if err != nil {
			return err
		}
		_, ok := currentInv[i]
		if ok {
			delete(currentInv, i)
		}
	}
	// If SKUs were omitted, set a default with unlimited inventry
	if len(listing.Item.Skus) == 0 {
		err = n.Datastore.Inventory().Put(listing.Slug, 0, -1)
		if err != nil {
			return err
		}
		_, ok := currentInv[0]
		if ok {
			delete(currentInv, 0)
		}
	}
	// Delete anything that did not update
	for i := range currentInv {
		err = n.Datastore.Inventory().Delete(listing.Slug, i)
		if err != nil {
			return err
		}
	}

	err = n.PublishInventory()
	if err != nil {
		return err
	}

	return nil
}

// CreateListing - add a listing
func (n *OpenBazaarNode) CreateListing(listing *pb.Listing) error {
	exists, err := n.listingExists(listing.Slug)
	if err != nil {
		return err
	}

	if exists {
		return ErrListingAlreadyExists
	}

	if listing.Slug == "" {
		listing.Slug, err = n.GenerateSlug(listing.Item.Title)
		if err != nil {
			return err
		}
	}

	return n.saveListing(listing)
}

// UpdateListing - update the listing
func (n *OpenBazaarNode) UpdateListing(listing *pb.Listing) error {
	exists, err := n.listingExists(listing.Slug)
	if err != nil {
		return err
	}

	if !exists {
		return ErrListingDoesNotExist
	}

	return n.saveListing(listing)
}

func (n *OpenBazaarNode) saveListing(listing *pb.Listing) error {
	if len(listing.Moderators) == 0 {
		sd, err := n.Datastore.Settings().Get()
		if err == nil && sd.StoreModerators != nil {
			listing.Moderators = *sd.StoreModerators
		}
	}

	if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		err := validateCryptocurrencyListing(listing)
		if err != nil {
			return err
		}

		setCryptocurrencyListingDefaults(listing)
	}

	err := n.SetListingInventory(listing)
	if err != nil {
		return err
	}

	signedListing, err := n.SignListing(listing)
	if err != nil {
		return err
	}

	f, err := os.Create(n.getPathForListingSlug(signedListing.Listing.Slug))
	if err != nil {
		return err
	}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(signedListing)
	if err != nil {
		return err
	}

	if _, err := f.WriteString(out); err != nil {
		return err
	}
	err = n.updateListingIndex(signedListing)
	if err != nil {
		return err
	}
	// Update followers/following
	err = n.UpdateFollow()
	if err != nil {
		return err
	}
	if err = n.SeedNode(); err != nil {
		return err
	}

	return nil
}

func (n *OpenBazaarNode) listingExists(slug string) (bool, error) {
	if slug == "" {
		return false, nil
	}
	_, ferr := os.Stat(n.getPathForListingSlug(slug))
	if slug == "" {
		return false, nil
	}
	if os.IsNotExist(ferr) {
		return false, nil
	}
	if ferr != nil {
		return false, ferr
	}
	return true, nil
}

func (n *OpenBazaarNode) getPathForListingSlug(slug string) string {
	return path.Join(n.RepoPath, "root", "listings", slug+".json")
}

func (n *OpenBazaarNode) updateListingIndex(listing *pb.SignedListing) error {
	ld, err := n.extractListingData(listing)
	if err != nil {
		return err
	}
	index, err := n.getListingIndex()
	if err != nil {
		return err
	}
	return n.updateListingOnDisk(index, ld, false)
}

func setCryptocurrencyListingDefaults(listing *pb.Listing) {
	listing.Coupons = []*pb.Listing_Coupon{}
	listing.Item.Options = []*pb.Listing_Item_Option{}
	listing.ShippingOptions = []*pb.Listing_ShippingOption{}
	listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
}

func coinDivisibilityForType(coinType string) uint32 {
	return DefaultCoinDivisibility
}

func (n *OpenBazaarNode) extractListingData(listing *pb.SignedListing) (ListingData, error) {
	listingPath := path.Join(n.RepoPath, "root", "listings", listing.Listing.Slug+".json")

	listingHash, err := ipfs.GetHashOfFile(n.IpfsNode, listingPath)
	if err != nil {
		return ListingData{}, err
	}

	descriptionLength := len(listing.Listing.Item.Description)
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
	for _, shippingOption := range listing.Listing.ShippingOptions {
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

	ld := ListingData{
		Hash:         listingHash,
		Slug:         listing.Listing.Slug,
		Title:        listing.Listing.Item.Title,
		Categories:   listing.Listing.Item.Categories,
		NSFW:         listing.Listing.Item.Nsfw,
		CoinType:     listing.Listing.Metadata.CoinType,
		ContractType: listing.Listing.Metadata.ContractType.String(),
		Description:  listing.Listing.Item.Description[:descriptionLength],
		Thumbnail:    thumbnail{listing.Listing.Item.Images[0].Tiny, listing.Listing.Item.Images[0].Small, listing.Listing.Item.Images[0].Medium},
		Price: price{
			CurrencyCode: listing.Listing.Metadata.PricingCurrency,
			Amount:       listing.Listing.Item.Price,
			Modifier:     listing.Listing.Metadata.PriceModifier,
		},
		ShipsTo:            shipsTo,
		FreeShipping:       freeShipping,
		Language:           listing.Listing.Metadata.Language,
		ModeratorIDs:       listing.Listing.Moderators,
		AcceptedCurrencies: []string{n.Wallet.CurrencyCode()},
	}
	return ld, nil
}

func (n *OpenBazaarNode) getListingIndex() ([]ListingData, error) {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")

	var index []ListingData

	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := ioutil.ReadFile(indexPath)
		if err != nil {
			return index, err
		}
		err = json.Unmarshal(file, &index)
		if err != nil {
			return index, err
		}
	}
	return index, nil
}

// Update the listings.json file in the listings directory
func (n *OpenBazaarNode) updateListingOnDisk(index []ListingData, ld ListingData, updateRatings bool) error {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")
	// Check to see if the listing we are adding already exists in the list. If so delete it.
	var avgRating float32
	var ratingCount uint32
	for i, d := range index {
		if d.Slug == ld.Slug {
			avgRating = d.AverageRating
			ratingCount = d.RatingCount

			if len(index) == 1 {
				index = []ListingData{}
				break
			}
			index = append(index[:i], index[i+1:]...)
			break
		}
	}

	// Append our listing with the new hash to the list
	if !updateRatings {
		ld.AverageRating = avgRating
		ld.RatingCount = ratingCount
	}
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

func (n *OpenBazaarNode) updateRatingInListingIndex(rating *pb.Rating) error {
	index, err := n.getListingIndex()
	if err != nil {
		return err
	}
	var ld ListingData
	exists := false
	for _, l := range index {
		if l.Slug == rating.RatingData.VendorSig.Metadata.ListingSlug {
			ld = l
			exists = true
			break
		}
	}
	if !exists {
		return errors.New("Listing for rating does not exist in index")
	}
	totalRating := ld.AverageRating * float32(ld.RatingCount)
	totalRating += float32(rating.RatingData.Overall)
	ld.AverageRating = totalRating / float32(ld.RatingCount+1)
	ld.RatingCount++
	return n.updateListingOnDisk(index, ld, true)
}

// UpdateEachListingOnIndex will visit each listing in the index and execute the function
// with a pointer to the listing passed as the argument. The function should return
// an error to further processing.
func (n *OpenBazaarNode) UpdateEachListingOnIndex(updateListing func(*ListingData) error) error {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")

	var index []ListingData

	_, ferr := os.Stat(indexPath)
	if os.IsNotExist(ferr) {
		return nil
	}
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, &index)
	if err != nil {
		return err
	}

	for i, d := range index {
		if err := updateListing(&d); err != nil {
			return err
		}
		index[i] = d
	}

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

// GetListingCount Return the current number of listings
func (n *OpenBazaarNode) GetListingCount() int {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")

	// Read existing file
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return 0
	}

	var index []ListingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return 0
	}
	return len(index)
}

// IsItemForSale Check to see we are selling the given listing. Used when validating an order.
// FIXME: This wont scale well. We will need to store the hash of active listings in a db to do an indexed search.
func (n *OpenBazaarNode) IsItemForSale(listing *pb.Listing) bool {
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		log.Error(err)
		return false
	}
	indexPath := path.Join(n.RepoPath, "root", "listings.json")

	// Read existing file
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		log.Error(err)
		return false
	}

	var index []ListingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		log.Error(err)
		return false
	}
	for _, l := range index {
		b, err := ipfs.Cat(n.IpfsNode, l.Hash, time.Minute)
		if err != nil {
			log.Error(err)
			return false
		}
		sl := new(pb.SignedListing)
		err = jsonpb.UnmarshalString(string(b), sl)
		if err != nil {
			log.Error(err)
			return false
		}
		ser, err := proto.Marshal(sl.Listing)
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

// DeleteListing Deletes the listing directory, removes the listing from the index, and deletes the inventory
func (n *OpenBazaarNode) DeleteListing(slug string) error {
	toDelete := path.Join(n.RepoPath, "root", "listings", slug+".json")
	err := os.Remove(toDelete)
	if err != nil {
		return err
	}
	var index []ListingData
	indexPath := path.Join(n.RepoPath, "root", "listings.json")
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

	// Check to see if the slug exists in the list. If so delete it.
	for i, d := range index {
		if d.Slug != slug {
			continue
		}

		if len(index) == 1 {
			index = []ListingData{}
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
	err = n.PublishInventory()
	if err != nil {
		return err
	}

	return n.updateProfileCounts()
}

// GetListings - fetch all listings
func (n *OpenBazaarNode) GetListings() ([]byte, error) {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")
	file, err := ioutil.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return []byte("[]"), nil
	} else if err != nil {
		return nil, err
	}

	// Unmarshal the index to check if file contains valid json
	var index []ListingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return nil, err
	}

	// Return bytes read from file
	return file, nil
}

// GetListingFromHash - fetch listing for the specified hash
func (n *OpenBazaarNode) GetListingFromHash(hash string) (*pb.SignedListing, error) {
	// Read listings.json
	indexPath := path.Join(n.RepoPath, "root", "listings.json")
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal the index
	var index []ListingData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return nil, err
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
		return nil, errors.New("Listing does not exist")
	}
	return n.GetListingFromSlug(slug)
}

// GetListingFromSlug - fetch listing for the specified slug
func (n *OpenBazaarNode) GetListingFromSlug(slug string) (*pb.SignedListing, error) {
	// Read listing file
	listingPath := path.Join(n.RepoPath, "root", "listings", slug+".json")
	file, err := ioutil.ReadFile(listingPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal listing
	sl := new(pb.SignedListing)
	err = jsonpb.UnmarshalString(string(file), sl)
	if err != nil {
		return nil, err
	}

	// Get the listing inventory
	inventory, err := n.Datastore.Inventory().Get(slug)
	if err != nil {
		return nil, err
	}

	// Build the inventory list
	for variant, count := range inventory {
		for i, s := range sl.Listing.Item.Skus {
			if variant == i {
				s.Quantity = count
				break
			}
		}
	}
	return sl, nil
}

/* Performs a ton of checks to make sure the listing is formatted correctly. We should not allow
   invalid listings to be saved or purchased as it can lead to ambiguity when moderating a dispute
   or possible attacks. This function needs to be maintained in conjunction with contracts.proto */
func validateListing(listing *pb.Listing, testnet bool) (err error) {
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
	if strings.Contains(listing.Slug, "/") {
		return errors.New("Slugs cannot contain file separators")
	}

	// Metadata
	if listing.Metadata == nil {
		return errors.New("Missing required field: Metadata")
	}
	if listing.Metadata.ContractType > pb.Listing_Metadata_CRYPTOCURRENCY {
		return errors.New("Invalid contract type")
	}
	if listing.Metadata.Format > pb.Listing_Metadata_MARKET_PRICE {
		return errors.New("Invalid listing format")
	}
	if listing.Metadata.Expiry == nil {
		return errors.New("Missing required field: Expiry")
	}
	if time.Unix(listing.Metadata.Expiry.Seconds, 0).Before(time.Now()) {
		return errors.New("Listing expiration must be in the future")
	}
	if len(listing.Metadata.Language) > WordMaxCharacters {
		return fmt.Errorf("Language is longer than the max of %d characters", WordMaxCharacters)
	}

	if !testnet && listing.Metadata.EscrowTimeoutHours != EscrowTimeout {
		return fmt.Errorf("Escrow timeout must be %d hours", EscrowTimeout)
	}
	if len(listing.Metadata.AcceptedCurrencies) == 0 {
		return errors.New("At least one accepted currency must be provided")
	}
	if len(listing.Metadata.AcceptedCurrencies) > MaxListItems {
		return fmt.Errorf("AcceptedCurrencies is longer than the max of %d currencies", MaxListItems)
	}
	for _, c := range listing.Metadata.AcceptedCurrencies {
		if len(c) > WordMaxCharacters {
			return fmt.Errorf("Accepted currency is longer than the max of %d characters", WordMaxCharacters)
		}
	}

	// Item
	if listing.Item.Title == "" {
		return errors.New("Listing must have a title")
	}
	if listing.Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY && listing.Item.Price == 0 {
		return errors.New("Zero price listings are not allowed")
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
		_, err := cid.Decode(img.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Small)
		if err != nil {
			return errors.New("Small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Large)
		if err != nil {
			return errors.New("Large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Original)
		if err != nil {
			return errors.New("Original image hashes must be properly formatted CID")
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

	maxCombos := 1
	variantSizeMap := make(map[int]int)
	optionMap := make(map[string]struct{})
	for i, option := range listing.Item.Options {
		if _, ok := optionMap[option.Name]; ok {
			return errors.New("Option names must be unique")
		}
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
		varMap := make(map[string]struct{})
		for _, variant := range option.Variants {
			if _, ok := varMap[variant.Name]; ok {
				return errors.New("Variant names must be unique")
			}
			if len(variant.Name) > WordMaxCharacters {
				return fmt.Errorf("Variant name length must be less than the max of %d", WordMaxCharacters)
			}
			if variant.Image != nil && (variant.Image.Filename != "" ||
				variant.Image.Large != "" || variant.Image.Medium != "" || variant.Image.Small != "" ||
				variant.Image.Tiny != "" || variant.Image.Original != "") {
				_, err := cid.Decode(variant.Image.Tiny)
				if err != nil {
					return errors.New("Tiny image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Small)
				if err != nil {
					return errors.New("Small image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Medium)
				if err != nil {
					return errors.New("Medium image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Large)
				if err != nil {
					return errors.New("Large image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Original)
				if err != nil {
					return errors.New("Original image hashes must be properly formatted CID")
				}
				if variant.Image.Filename == "" {
					return errors.New("Image file names must not be nil")
				}
				if len(variant.Image.Filename) > FilenameMaxCharacters {
					return fmt.Errorf("Image filename length must be less than the max of %d", FilenameMaxCharacters)
				}
			}
			varMap[variant.Name] = struct{}{}
		}
		variantSizeMap[i] = len(option.Variants)
		maxCombos *= len(option.Variants)
		optionMap[option.Name] = struct{}{}
	}

	if len(listing.Item.Skus) > maxCombos {
		return errors.New("More skus than variant combinations")
	}
	comboMap := make(map[string]bool)
	for _, sku := range listing.Item.Skus {
		if maxCombos > 1 && len(sku.VariantCombo) == 0 {
			return errors.New("Skus must specify a variant combo when options are used")
		}
		if len(sku.ProductID) > WordMaxCharacters {
			return fmt.Errorf("Product ID length must be less than the max of %d", WordMaxCharacters)
		}
		formatted, err := json.Marshal(sku.VariantCombo)
		if err != nil {
			return err
		}
		_, ok := comboMap[string(formatted)]
		if !ok {
			comboMap[string(formatted)] = true
		} else {
			return errors.New("Duplicate sku")
		}
		if len(sku.VariantCombo) != len(listing.Item.Options) {
			return errors.New("Incorrect number of variants in sku combination")
		}
		for i, combo := range sku.VariantCombo {
			if int(combo) > variantSizeMap[i] {
				return errors.New("Invalid sku variant combination")
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
			return errors.New("Tax must specify at least one region")
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
		if len(coupon.Title) > CouponTitleMaxCharacters {
			return fmt.Errorf("Coupon title length must be less than the max of %d", SentenceMaxCharacters)
		}
		if len(coupon.GetDiscountCode()) > CodeMaxCharacters {
			return fmt.Errorf("Coupon code length must be less than the max of %d", CodeMaxCharacters)
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

	// Type-specific validations
	if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
		err := validatePhysicalListing(listing)
		if err != nil {
			return err
		}
	} else if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		err := validateCryptocurrencyListing(listing)
		if err != nil {
			return err
		}
	}

	// Format-specific validations
	if listing.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
		err := validateMarketPriceListing(listing)
		if err != nil {
			return err
		}
	}

	return nil
}

func validatePhysicalListing(listing *pb.Listing) error {
	if listing.Metadata.PricingCurrency == "" {
		return errors.New("Listing pricing currency code must not be empty")
	}
	if len(listing.Metadata.PricingCurrency) > WordMaxCharacters {
		return fmt.Errorf("PricingCurrency is longer than the max of %d characters", WordMaxCharacters)
	}
	if len(listing.Item.Condition) > SentenceMaxCharacters {
		return fmt.Errorf("Condition length must be less than the max of %d", SentenceMaxCharacters)
	}
	if len(listing.Item.Options) > MaxListItems {
		return fmt.Errorf("Number of options is greater than the max of %d", MaxListItems)
	}

	// ShippingOptions
	if len(listing.ShippingOptions) == 0 {
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
			return errors.New("Unknown shipping option type")
		}
		if len(shippingOption.Regions) == 0 {
			return errors.New("Shipping options must specify at least one region")
		}
		for _, region := range shippingOption.Regions {
			if int(region) == 0 {
				return errors.New("Shipping region cannot be NA")
			} else if int(region) > 246 && int(region) != 500 {
				return errors.New("Invalid shipping region")
			}

		}
		if len(shippingOption.Regions) > MaxCountryCodes {
			return fmt.Errorf("Number of shipping regions is greater than the max of %d", MaxCountryCodes)
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

	return nil
}

func validateCryptocurrencyListing(listing *pb.Listing) error {
	switch {
	case len(listing.Coupons) > 0:
		return ErrCryptocurrencyListingIllegalField("coupons")
	case len(listing.Item.Options) > 0:
		return ErrCryptocurrencyListingIllegalField("item.options")
	case len(listing.ShippingOptions) > 0:
		return ErrCryptocurrencyListingIllegalField("shippingOptions")
	case len(listing.Item.Condition) > 0:
		return ErrCryptocurrencyListingIllegalField("item.condition")
	case len(listing.Metadata.PricingCurrency) > 0:
		return ErrCryptocurrencyListingIllegalField("metadata.pricingCurrency")
	case listing.Metadata.CoinType == "":
		return ErrCryptocurrencyListingCoinTypeRequired
	}

	if listing.Metadata.CoinDivisibility != coinDivisibilityForType(listing.Metadata.CoinType) {
		return ErrListingCoinDivisibilityIncorrect
	}

	return nil
}

func validateMarketPriceListing(listing *pb.Listing) error {
	if listing.Item.Price > 0 {
		return ErrMarketPriceListingIllegalField("item.price")
	}

	if listing.Metadata.PriceModifier != 0 {
		listing.Metadata.PriceModifier = float32(int(listing.Metadata.PriceModifier*100.0)) / 100.0
	}

	if listing.Metadata.PriceModifier < PriceModifierMin ||
		listing.Metadata.PriceModifier > PriceModifierMax {
		return ErrPriceModifierOutOfRange{
			Min: PriceModifierMin,
			Max: PriceModifierMax,
		}
	}

	return nil
}

func validateListingSkus(listing *pb.Listing) error {
	if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		for _, sku := range listing.Item.Skus {
			if sku.Quantity < 1 {
				return ErrCryptocurrencySkuQuantityInvalid
			}
		}
	}
	return nil
}

func verifySignaturesOnListing(sl *pb.SignedListing) error {
	// Verify identity signature on listing
	if err := verifySignature(
		sl.Listing,
		sl.Listing.VendorID.Pubkeys.Identity,
		sl.Signature,
		sl.Listing.VendorID.PeerID,
	); err != nil {
		switch err.(type) {
		case invalidSigError:
			return errors.New("Vendor's identity signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in order does not match reported buyer ID")
		default:
			return err
		}
	}

	// Verify the bitcoin signature in the ID
	if err := verifyBitcoinSignature(
		sl.Listing.VendorID.Pubkeys.Bitcoin,
		sl.Listing.VendorID.BitcoinSig,
		sl.Listing.VendorID.PeerID,
	); err != nil {
		switch err.(type) {
		case invalidSigError:
			return errors.New("Vendor's bitcoin signature on GUID failed to verify")
		default:
			return err
		}
	}
	return nil
}
