package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type price struct {
	CurrencyCode string             `json:"currencyCode"`
	Amount       repo.CurrencyValue `json:"amount"`
	Modifier     float32            `json:"modifier"`
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

// SignListing Add our identity to the listing and sign it
func (n *OpenBazaarNode) SignListing(listing *repo.Listing) (*repo.SignedListing, error) {
	timeout := uint32(0)
	// Temporary hack to work around test env shortcomings
	if n.TestNetworkEnabled() || n.RegressionNetworkEnabled() {
		//
	} else {
		timeout = repo.EscrowTimeout
	}
	profile, err := n.GetProfile()
	handle := ""
	if err == nil {
		handle = profile.Handle
	}
	currencyMap := make(map[string]bool)
	currencies, err := listing.GetAcceptedCurrencies()
	if err != nil {
		return nil, err
	}
	for _, acceptedCurrency := range currencies {
		_, err := n.Multiwallet.WalletForCurrencyCode(acceptedCurrency)
		if err != nil {
			return nil, fmt.Errorf("currency %s is not found in multiwallet", acceptedCurrency)
		}
		if currencyMap[NormalizeCurrencyCode(acceptedCurrency)] {
			return nil, errors.New("duplicate accepted currency in listing")
		}
		currencyMap[NormalizeCurrencyCode(acceptedCurrency)] = true
	}
	var expectedDivisibility uint32
	currencyVal, err := listing.GetPrice()
	if err != nil {
		return nil, err
	}
	if wallet, err := n.Multiwallet.WalletForCurrencyCode(currencyVal.Currency.Code.String()); err != nil {
		expectedDivisibility = DefaultCurrencyDivisibility
	} else {
		expectedDivisibility = uint32(math.Log10(float64(wallet.ExchangeRates().UnitsPerCoin())))
	}
	return listing.Sign(n.IpfsNode, timeout, expectedDivisibility, handle, n.MasterPrivateKey, &n.Datastore)
}

/*SetListingInventory Sets the inventory for the listing in the database. Does some basic validation
  to make sure the inventory uses the correct variants. */
func (n *OpenBazaarNode) SetListingInventory(l *repo.Listing) error {
	err := l.ValidateSkus()
	if err != nil {
		return err
	}
	slug, err := l.GetSlug()
	if err != nil {
		return err
	}

	// Grab current inventory
	currentInv, err := n.Datastore.Inventory().Get(slug)
	if err != nil {
		return err
	}
	// Get the listing inventory
	listingInv, err := l.GetInventory()
	if err != nil {
		return err
	}

	// Update inventory
	for i, s := range listingInv {
		err = n.Datastore.Inventory().Put(slug, i, s)
		if err != nil {
			return err
		}
		_, ok := currentInv[i]
		if ok {
			delete(currentInv, i)
		}
	}
	// If SKUs were omitted, set a default with unlimited inventry
	if len(listingInv) == 0 {
		err = n.Datastore.Inventory().Put(slug, 0, -1)
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
		err = n.Datastore.Inventory().Delete(slug, i)
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
func (n *OpenBazaarNode) CreateListing(r io.Reader) (string, error) {
	listing, err := repo.CreateListing(r, n.TestNetworkEnabled(), &n.Datastore)
	if err != nil {
		return "", err
	}
	return listing.ProtoListing.Slug, n.saveListing(listing, true)
}

// UpdateListing - update the listing
func (n *OpenBazaarNode) UpdateListing(r io.Reader, publish bool) error {
	listing, err := repo.UpdateListing(r, n.TestNetworkEnabled(), &n.Datastore)
	if err != nil {
		return err
	}
	return n.saveListing(listing, publish)
}

func (n *OpenBazaarNode) getExpectedDivisibility(code string) uint32 {
	var expectedDivisibility uint32
	if wallet, err := n.Multiwallet.WalletForCurrencyCode(code); err != nil {
		expectedDivisibility = DefaultCurrencyDivisibility
	} else {
		expectedDivisibility = uint32(math.Log10(float64(wallet.ExchangeRates().UnitsPerCoin())))
	}
	return expectedDivisibility
}

func prepListingForPublish(n *OpenBazaarNode, listing *repo.Listing) error {
	mods, err := listing.GetModerators()
	if err != nil {
		return err
	}
	if len(mods) == 0 {
		sd, err := n.Datastore.Settings().Get()
		if err == nil && sd.StoreModerators != nil {
			listing.SetModerators(*sd.StoreModerators)
		}
	}

	ct, err := listing.GetContractType()
	if err != nil {
		return err
	}
	if pb.Listing_Metadata_ContractType_value[ct] == int32(pb.Listing_Metadata_CRYPTOCURRENCY) {
		currencyVal, err := listing.GetPrice()
		if err != nil {
			return err
		}
		expectedDivisibility := n.getDivisibility(currencyVal.Currency.Code.String())
		err = listing.ValidateCryptoListing(expectedDivisibility)
		if err != nil {
			return err
		}

		err = listing.SetCryptocurrencyListingDefaults()
		if err != nil {
			return err
		}
	}

	err = n.SetListingInventory(listing)
	if err != nil {
		return err
	}

	err = n.maybeMigrateImageHashes(listing.ProtoListing)
	if err != nil {
		return err
	}
	listing, err = repo.NewListingFromProtobuf(listing.ProtoListing)
	if err != nil {
		return err
	}

	signedListing, err := n.SignListing(listing)
	if err != nil {
		return err
	}

	fName, err := repo.GetPathForListingSlug(signedListing.Listing.Slug, n.TestNetworkEnabled())
	if err != nil {
		return err
	}
	f, err := os.Create(fName)
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
	err = n.updateListingIndex(signedListing.ProtoSignedListing)
	if err != nil {
		return err
	}

	return nil
}

func (n *OpenBazaarNode) saveListing(listing *repo.Listing, publish bool) error {

	err := prepListingForPublish(n, listing)
	if err != nil {
		return err
	}

	// Update followers/following
	err = n.UpdateFollow()
	if err != nil {
		return err
	}

	if publish {
		if err = n.SeedNode(); err != nil {
			return err
		}
	}

	return nil
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

func (n *OpenBazaarNode) extractListingData(listing *pb.SignedListing) (ListingData, error) {
	listingPath := path.Join(n.RepoPath, "root", "listings", listing.Listing.Slug+".json")

	listingHash, err := ipfs.GetHashOfFile(n.IpfsNode, listingPath)
	if err != nil {
		return ListingData{}, err
	}

	descriptionLength := len(listing.Listing.Item.Description)
	if descriptionLength > repo.ShortDescriptionLength {
		descriptionLength = repo.ShortDescriptionLength
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
				servicePrice, _ := new(big.Int).SetString(service.PriceValue.Amount, 10)
				if servicePrice.Cmp(big.NewInt(0)) == 0 && !contains(freeShipping, region.String()) {
					freeShipping = append(freeShipping, region.String())
				}
			}
		}
	}

	defn, _ := repo.LoadCurrencyDefinitions().Lookup(listing.Listing.Metadata.PricingCurrencyDefn.Code)
	amt, _ := new(big.Int).SetString(listing.Listing.Item.PriceValue.Amount, 10)

	ld := ListingData{
		Hash:         listingHash,
		Slug:         listing.Listing.Slug,
		Title:        listing.Listing.Item.Title,
		Categories:   listing.Listing.Item.Categories,
		NSFW:         listing.Listing.Item.Nsfw,
		CoinType:     listing.Listing.Metadata.PricingCurrencyDefn.Code,
		ContractType: listing.Listing.Metadata.ContractType.String(),
		Description:  listing.Listing.Item.Description[:descriptionLength],
		Thumbnail:    thumbnail{listing.Listing.Item.Images[0].Tiny, listing.Listing.Item.Images[0].Small, listing.Listing.Item.Images[0].Medium},
		Price: price{
			CurrencyCode: listing.Listing.Metadata.PricingCurrencyDefn.Code,
			Amount:       repo.CurrencyValue{Currency: defn, Amount: amt},
			Modifier:     listing.Listing.Metadata.PriceModifier,
		},
		ShipsTo:            shipsTo,
		FreeShipping:       freeShipping,
		Language:           listing.Listing.Metadata.Language,
		ModeratorIDs:       listing.Listing.Moderators,
		AcceptedCurrencies: listing.Listing.Metadata.AcceptedCurrencies,
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
		return errors.New("listing for rating does not exist in index")
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
// FIXME: This won't scale well. We will need to store the hash of active listings in a db to do an indexed search.
func (n *OpenBazaarNode) IsItemForSale(listing *pb.Listing) bool {
	var log = logging.MustGetLogger("core")
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
		return nil, errors.New("listing does not exist")
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

func verifySignaturesOnListing(sl *repo.SignedListing) error {
	// Verify identity signature on listing
	if err := verifySignature(
		sl.Listing.ProtoListing,
		sl.Listing.Vendor.Protobuf().Pubkeys.Identity,
		sl.Signature,
		sl.Listing.Vendor.Protobuf().PeerID,
	); err != nil {
		switch err.(type) {
		case invalidSigError:
			return errors.New("vendor's identity signature on contact failed to verify")
		case matchKeyError:
			return errors.New("public key in order does not match reported buyer ID")
		default:
			return err
		}
	}

	// Verify the bitcoin signature in the ID
	if err := verifyBitcoinSignature(
		sl.Listing.Vendor.Protobuf().Pubkeys.Bitcoin,
		sl.Listing.Vendor.Protobuf().BitcoinSig,
		sl.Listing.Vendor.Protobuf().PeerID,
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

// SetCurrencyOnListings - set currencies accepted for a listing
func (n *OpenBazaarNode) SetCurrencyOnListings(currencies []string) error {
	absPath, err := filepath.Abs(path.Join(n.RepoPath, "root", "listings"))
	if err != nil {
		return err
	}

	walkpath := func(p string, f os.FileInfo, err error) error {
		if !f.IsDir() && filepath.Ext(p) == ".json" {

			sl, err := GetSignedListingFromPath(p)
			if err != nil {
				return err
			}

			// Cryptocurrency listings can only have one currency listed and since it's
			// a trade for one specific currency for another specific currency it isn't
			// appropriate to apply the bulk update to this type of listing.
			if sl.Listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
				return nil
			}

			SetAcceptedCurrencies(sl, currencies)

			savedCoupons, err := n.Datastore.Coupons().Get(sl.Listing.Slug)
			if err != nil {
				return err
			}
			err = AssignMatchingCoupons(savedCoupons, sl)
			if err != nil {
				return err
			}

			if sl.Listing.Metadata != nil && sl.Listing.Metadata.Version == 1 {
				err = ApplyShippingOptions(sl)
				if err != nil {
					return err
				}
			}

			inventory, err := n.Datastore.Inventory().Get(sl.Listing.Slug)
			if err != nil {
				return err
			}
			err = AssignMatchingQuantities(inventory, sl)
			if err != nil {
				return err
			}

			rListing, err := repo.NewListingFromProtobuf(sl.Listing)
			if err != nil {
				return err
			}
			err = n.UpdateListing(rListing, false)
			if err != nil {
				return err
			}

		}
		return nil
	}

	err = filepath.Walk(absPath, walkpath)
	if err != nil {
		return err
	}

	err = n.SeedNode()
	if err != nil {
		return err
	}

	return nil
}
