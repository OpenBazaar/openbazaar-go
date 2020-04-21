package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

// SetListingInventory sets the inventory for the listing in the database. Does some basic validation
// to make sure the inventory uses the correct variants.
func (n *OpenBazaarNode) SetListingInventory(l repo.Listing) error {
	err := l.ValidateSkus()
	if err != nil {
		return err
	}

	// Grab current inventory
	currentInv, err := n.Datastore.Inventory().Get(l.GetSlug())
	if err != nil {
		return err
	}
	// Get the listing inventory
	listingInv, err := l.GetInventory()
	if err != nil {
		return err
	}

	// If SKUs were omitted, set a default with unlimited inventory
	if len(listingInv) == 0 {
		err = n.Datastore.Inventory().Put(l.GetSlug(), 0, big.NewInt(-1))
		if err != nil {
			return err
		}
		_, ok := currentInv[0]
		if ok {
			delete(currentInv, 0)
		}
	} else {
		// Update w provided inventory
		for i, s := range listingInv {
			err = n.Datastore.Inventory().Put(l.GetSlug(), i, s)
			if err != nil {
				return err
			}
			_, ok := currentInv[i]
			if ok {
				delete(currentInv, i)
			}
		}
	}

	// Delete anything that did not update
	for i := range currentInv {
		err = n.Datastore.Inventory().Delete(l.GetSlug(), i)
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
func (n *OpenBazaarNode) CreateListing(r []byte) (string, error) {
	listing, err := repo.CreateListing(r, n.TestNetworkEnabled() || n.RegressionNetworkEnabled(), &n.Datastore, n.RepoPath)
	if err != nil {
		return "", err
	}
	return listing.GetSlug(), n.saveListing(listing, true)
}

// UpdateListing - update the listing
func (n *OpenBazaarNode) UpdateListing(r []byte, publish bool) error {
	listing, err := repo.UpdateListing(r, n.TestNetworkEnabled() || n.RegressionNetworkEnabled(), &n.Datastore, n.RepoPath)
	if err != nil {
		return err
	}
	return n.saveListing(listing, publish)
}

func (n *OpenBazaarNode) validateListingIsSellable(l repo.Listing) error {
	var isTestnet = n.TestNetworkEnabled() || n.RegressionNetworkEnabled()

	if err := l.ValidateListing(isTestnet); err != nil {
		return err
	}

	var acceptedCurrenciesSeen = make(map[string]bool)
	for _, c := range l.GetAcceptedCurrencies() {
		if _, err := n.Multiwallet.WalletForCurrencyCode(c); err != nil {
			return fmt.Errorf("currency (%s) not supported by wallet", c)
		}

		currDef, err := repo.AllCurrencies().Lookup(c)
		if err != nil {
			return fmt.Errorf("lookup currency (%s): %s", c, err.Error())
		}

		if acceptedCurrenciesSeen[currDef.CurrencyCode().String()] {
			return errors.New("duplicate accepted currency in listing")
		}
		acceptedCurrenciesSeen[currDef.CurrencyCode().String()] = true
	}

	return nil
}

func (n *OpenBazaarNode) saveListing(l repo.Listing, publish bool) error {
	mods := l.GetModerators()
	if len(mods) == 0 {
		sd, err := n.Datastore.Settings().Get()
		if err == nil && sd.StoreModerators != nil {
			if err := l.SetModerators(*sd.StoreModerators); err != nil {
				return err
			}
		}
	}

	ct := l.GetContractType()
	if pb.Listing_Metadata_ContractType_value[ct] == int32(pb.Listing_Metadata_CRYPTOCURRENCY) {
		if err := l.ValidateCryptoListing(); err != nil {
			return err
		}

		if err := l.SetCryptocurrencyListingDefaults(); err != nil {
			return err
		}
	}

	if err := n.validateListingIsSellable(l); err != nil {
		return fmt.Errorf("validate sellable listing (%s): %s", l.GetSlug(), err.Error())
	}

	if err := n.SetListingInventory(l); err != nil {
		return err
	}

	if err := n.maybeMigrateImageHashes(&l); err != nil {
		return err
	}

	// Update coupon db
	if err := n.updateListingCoupons(&l); err != nil {
		return fmt.Errorf("updating (%s) coupons: %s", l.GetSlug(), err.Error())
	}

	sl, err := l.Sign(n)
	if err != nil {
		return err
	}

	fName, err := repo.GetPathForListingSlug(sl.GetSlug(), n.RepoPath, n.TestNetworkEnabled())
	if err != nil {
		return err
	}
	f, err := os.Create(fName)
	if err != nil {
		return err
	}
	defer f.Close()

	out, err := sl.MarshalJSON()
	if err != nil {
		return err
	}
	if _, err := f.Write(out); err != nil {
		return err
	}

	ld, err := n.toListingIndexData(&l)
	if err != nil {
		return err
	}
	index, err := n.getListingIndex()
	if err != nil {
		return err
	}
	err = n.updateListingOnDisk(index, ld, false)
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

func (n *OpenBazaarNode) updateListingCoupons(l *repo.Listing) error {
	cs, err := l.GetCoupons()
	if err != nil {
		return fmt.Errorf("get coupons: %s", err.Error())
	}
	var couponsToSave = make([]repo.Coupon, 0)
	for _, c := range cs {
		// check for redemption code and only persist if available
		cCode, err := c.GetRedemptionCode()
		if err != nil {
			log.Warningf("not persisting coupon (%s): missing redemption code", c.GetTitle())
			continue
		}

		cHash, err := c.GetRedemptionHash()
		if err != nil {
			return fmt.Errorf("get redemption hash (%s): %s", c.GetTitle(), err.Error())
		}
		couponsToSave = append(couponsToSave, repo.Coupon{
			Slug: l.GetSlug(),
			Code: cCode,
			Hash: cHash,
		})
	}
	if len(couponsToSave) > 0 {
		// check if some coupons have codes but not others to avoid missing coupons
		if len(couponsToSave) != len(cs) {
			return fmt.Errorf("not all coupons for listing (%s) could be persisted due to missing redemption codes", l.GetSlug())
		}
		if err := n.Datastore.Coupons().Delete(l.GetSlug()); err != nil {
			log.Errorf("failed removing old coupons for listing (%s): %s", l.GetSlug(), err.Error())
		}
		if err := n.Datastore.Coupons().Put(couponsToSave); err != nil {
			return fmt.Errorf("persisting coupons: %s", err.Error())
		}
	}
	return nil
}

func (n *OpenBazaarNode) toListingIndexData(l *repo.Listing) (repo.ListingIndexData, error) {
	var (
		listingPath        = path.Join(n.RepoPath, "root", "listings", l.GetSlug()+".json")
		shipTo, freeShipTo = l.GetShippingRegions()
		previewImg         = l.GetImages()[0]
	)

	listingHash, err := ipfs.GetHashOfFile(n.IpfsNode, listingPath)
	if err != nil {
		return repo.ListingIndexData{}, fmt.Errorf("get hash: %s", err.Error())
	}
	priceValue, err := l.GetPrice()
	if err != nil {
		return repo.ListingIndexData{}, fmt.Errorf("get price: %s", err.Error())
	}

	return repo.ListingIndexData{
		Hash:         listingHash,
		Slug:         l.GetSlug(),
		Title:        l.GetTitle(),
		Categories:   l.GetCategories(),
		NSFW:         l.GetNsfw(),
		ContractType: l.GetContractType(),
		Description:  l.GetShortDescription(),
		Thumbnail: repo.ListingThumbnail{
			previewImg.GetTiny(),
			previewImg.GetSmall(),
			previewImg.GetMedium(),
		},
		Price:              priceValue,
		Modifier:           l.GetPriceModifier(),
		ShipsTo:            shipTo,
		FreeShipping:       freeShipTo,
		Language:           l.GetLanguage(),
		ModeratorIDs:       l.GetModerators(),
		AcceptedCurrencies: l.GetAcceptedCurrencies(),
		CryptoCurrencyCode: l.GetCryptoCurrencyCode(),
	}, nil
}

func (n *OpenBazaarNode) getListingIndex() ([]repo.ListingIndexData, error) {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")

	var index []repo.ListingIndexData

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
func (n *OpenBazaarNode) updateListingOnDisk(index []repo.ListingIndexData, ld repo.ListingIndexData, updateRatings bool) error {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")
	// Check to see if the listing we are adding already exists in the list. If so delete it.
	var avgRating float32
	var ratingCount uint32
	for i, d := range index {
		if d.Slug == ld.Slug {
			avgRating = d.AverageRating
			ratingCount = d.RatingCount

			if len(index) == 1 {
				index = []repo.ListingIndexData{}
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
	var ld repo.ListingIndexData
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
func (n *OpenBazaarNode) UpdateEachListingOnIndex(updateListing func(*repo.ListingIndexData) error) error {
	indexPath := path.Join(n.RepoPath, "root", "listings.json")

	var index []repo.ListingIndexData

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

	var index []repo.ListingIndexData
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

	var index []repo.ListingIndexData
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
	var index []repo.ListingIndexData
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
			index = []repo.ListingIndexData{}
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
	var index []repo.ListingIndexData
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
	var index []repo.ListingIndexData
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
		for i := range sl.Listing.Item.Skus {
			if variant == i {
				sl.Listing.Item.Skus[i].BigQuantity = count.String()
				break
			}
		}
	}

	for _, s := range sl.Listing.Item.Skus {
		if s.BigSurcharge == "" {
			s.BigSurcharge = "0"
		}
	}

	return sl, nil
}

// SetCurrencyOnListings - set currencies accepted for a listing
func (n *OpenBazaarNode) SetCurrencyOnListings(currencies []string) error {
	absPath, err := filepath.Abs(path.Join(n.RepoPath, "root", "listings"))
	if err != nil {
		return err
	}
	walkpath := func(p string, f os.FileInfo, err error) error {
		if !f.IsDir() && filepath.Ext(p) == ".json" {
			signedProto, err := GetSignedListingFromPath(p)
			if err != nil {
				return err
			}

			oldSL := repo.NewSignedListingFromProtobuf(signedProto)
			l := oldSL.GetListing()

			// Cryptocurrency listings can only have one currency listed and since it's
			// a trade for one specific currency for another specific currency it isn't
			// appropriate to apply the bulk update to this type of listing.
			if l.GetContractType() == pb.Listing_Metadata_CRYPTOCURRENCY.String() {
				return nil
			}
			if err := l.SetAcceptedCurrencies(currencies...); err != nil {
				return err
			}

			lb, err := l.MarshalJSON()
			if err != nil {
				return fmt.Errorf("marshaling signed listing (%s): %s", l.GetSlug(), err.Error())
			}
			err = n.UpdateListing(lb, false)
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
