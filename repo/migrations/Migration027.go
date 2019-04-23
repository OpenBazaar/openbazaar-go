package migrations

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"

	"github.com/OpenBazaar/jsonpb"
	"github.com/golang/protobuf/proto"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

const migration027_ListingVersionForMarkupListings = 5

type Migration027 struct{}

type Migration027_ListingData struct {
	Hash         string   `json:"hash"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Categories   []string `json:"categories"`
	NSFW         bool     `json:"nsfw"`
	ContractType string   `json:"contractType"`
	Description  string   `json:"description"`
	Thumbnail    struct {
		Tiny   string `json:"tiny"`
		Small  string `json:"small"`
		Medium string `json:"medium"`
	} `json:"thumbnail"`
	Price struct {
		CurrencyCode string  `json:"currencyCode"`
		Amount       uint64  `json:"amount"`
		Modifier     float32 `json:"modifier"`
	} `json:"price"`
	ShipsTo            []string `json:"shipsTo"`
	FreeShipping       []string `json:"freeShipping"`
	Language           string   `json:"language"`
	AverageRating      float32  `json:"averageRating"`
	RatingCount        uint32   `json:"ratingCount"`
	ModeratorIDs       []string `json:"moderators"`
	AcceptedCurrencies []string `json:"acceptedCurrencies"`
	CoinType           string   `json:"coinType"`
	Item               struct {
		Price uint64 `json:"price"`
		Skus  []struct {
			ProductID string `json:"productID,omitempty"`
			Surcharge int64  `json:"surcharge,omitempty"`
		} `json:"skus,omitempty"`
	} `json:"item"`
	ShippingOptions []struct {
		Name     string `json:"name,omitempty"`
		Services []struct {
			Name       string `json:"name,omitempty"`
			Price      uint64 `json:"price,omitempty"`
			AddlnPrice uint64 `json:"additionalItemPrice,omitempty"`
		} `json:"services,omitempty"`
	} `json:"shippingOptions,omitempty"`
	Coupons []struct {
		Title    string `json:"title,omitempty"`
		Discount struct {
			Price uint64 `json:"priceDiscount,omitempty"`
		} `json:"discount,omitempty"`
	} `json:"coupons,omitempty"`
}

func Migration027_GetIdentityKey(repoPath, databasePassword string, testnetEnabled bool) ([]byte, error) {
	db, err := OpenDB(repoPath, databasePassword, testnetEnabled)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var identityKey []byte
	err = db.
		QueryRow("select value from config where key=?", "identityKey").
		Scan(&identityKey)
	if err != nil {
		return nil, err
	}
	return identityKey, nil
}

func (Migration027) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	listingsIndexFilePath := path.Join(repoPath, "root", "listings.json")

	// Find all crypto listings
	if _, err := os.Stat(listingsIndexFilePath); os.IsNotExist(err) {
		// Finish early if no listings are found
		return writeRepoVer(repoPath, 28)
	}
	listingsIndexJSONBytes, err := ioutil.ReadFile(listingsIndexFilePath)
	if err != nil {
		return err
	}

	var listingsIndex []*Migration027_ListingData
	err = json.Unmarshal(listingsIndexJSONBytes, &listingsIndex)
	if err != nil {
		return err
	}

	var cryptoListings []*Migration027_ListingData

	cryptoListings = append(cryptoListings, listingsIndex...)

	// Finish early If no crypto listings
	if len(cryptoListings) == 0 {
		return writeRepoVer(repoPath, 28)
	}

	// Check each crypto listing for markup
	var markupListings []*pb.SignedListing
	for _, listingAbstract := range cryptoListings {
		listingJSONBytes, err := ioutil.ReadFile(migration027_listingFilePath(repoPath, listingAbstract.Slug))
		if err != nil {
			return err
		}
		sl := new(pb.SignedListing)
		err = jsonpb.UnmarshalString(string(listingJSONBytes), sl)
		if err != nil {
			return err
		}

		sl.Listing.Metadata.PricingCurrency = &pb.CurrencyDefinition{
			Code:         listingAbstract.Price.CurrencyCode,
			Divisibility: 8,
		}

		sl.Listing.Item.Price = &pb.CurrencyValue{
			Currency: sl.Listing.Metadata.PricingCurrency,
			Value:    strconv.FormatUint(listingAbstract.Price.Amount, 10),
		}

		for _, sku := range listingAbstract.Item.Skus {
			for j, s := range sl.Listing.Item.Skus {
				if s.ProductID != sku.ProductID {
					continue
				}
				sl.Listing.Item.Skus[j].Surcharge = &pb.CurrencyValue{
					Currency: sl.Listing.Metadata.PricingCurrency,
					Value:    strconv.FormatInt(sku.Surcharge, 10),
				}
			}
		}

		for _, so := range listingAbstract.ShippingOptions {
			for i, so1 := range sl.Listing.ShippingOptions {
				if so.Name != so1.Name {
					continue
				}
				for _, ser := range so.Services {
					for j, ser0 := range sl.Listing.ShippingOptions[i].Services {
						if ser.Name != ser0.Name {
							continue
						}
						sl.Listing.ShippingOptions[i].Services[j].Price = &pb.CurrencyValue{
							Currency: sl.Listing.Metadata.PricingCurrency,
							Value:    strconv.FormatUint(ser.Price, 10),
						}
					}
				}
			}
		}

		for _, coupon := range listingAbstract.Coupons {
			for i, c0 := range sl.Listing.Coupons {
				if coupon.Title != c0.Title {
					continue
				}
				if c0.Discount != nil {
					switch c0.Discount.(type) {
					case *pb.Listing_Coupon_PriceDiscount:
						{
							sl.Listing.Coupons[i].Discount.(*pb.Listing_Coupon_PriceDiscount).PriceDiscount = &pb.CurrencyValue{
								Currency: sl.Listing.Metadata.PricingCurrency,
								Value:    strconv.FormatUint(coupon.Discount.Price, 10),
							}
						}
					}
				}
			}
		}

		markupListings = append(markupListings, sl)

	}

	// Finish early If no crypto listings with new features are found
	if len(markupListings) == 0 {
		return writeRepoVer(repoPath, 28)
	}

	// Setup signing capabilities
	identityKey, err := Migration027_GetIdentityKey(repoPath, databasePassword, testnetEnabled)
	if err != nil {
		return err
	}

	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	// IPFS node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := r.Config()
	if err != nil {
		return err
	}

	cfg.Identity = identity

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: false,
		ExtraOpts: map[string]bool{
			"mplex": true,
		},
		Routing: nil,
	}

	nd, err := ipfscore.NewNode(cctx, ncfg)
	if err != nil {
		return err
	}
	defer nd.Close()

	// Update each listing to have the latest version number and resave
	// Save the new hashes for each changed listing so we can update the index.
	hashes := make(map[string]string)

	privKey, err := crypto.UnmarshalPrivateKey(identityKey)
	if err != nil {
		return err
	}

	for _, sl := range markupListings {

		serializedListing, err := proto.Marshal(sl.Listing)
		if err != nil {
			return err
		}

		idSig, err := privKey.Sign(serializedListing)
		if err != nil {
			return err
		}
		sl.Signature = idSig

		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		out, err := m.MarshalToString(sl)
		if err != nil {
			return err
		}

		filename := migration027_listingFilePath(repoPath, sl.Listing.Slug)
		if err := ioutil.WriteFile(filename, []byte(out), os.ModePerm); err != nil {
			return err
		}
		h, err := ipfs.GetHashOfFile(nd, filename)
		if err != nil {
			return err
		}
		hashes[sl.Listing.Slug] = h
	}

	// Update listing index
	indexBytes, err := ioutil.ReadFile(listingsIndexFilePath)
	if err != nil {
		return err
	}
	var index []Migration027_ListingData

	err = json.Unmarshal(indexBytes, &index)
	if err != nil {
		return err
	}

	for i, l := range index {
		h, ok := hashes[l.Slug]

		// Not one of the changed listings
		if !ok {
			continue
		}

		l.Hash = h
		index[i] = l
	}

	// Write it back to file
	ifile, err := os.Create(listingsIndexFilePath)
	if err != nil {
		return err
	}
	defer ifile.Close()

	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := ifile.Write(j)
	if werr != nil {
		return werr
	}

	_, err = ipfs.GetHashOfFile(nd, listingsIndexFilePath)
	if err != nil {
		return err
	}

	return writeRepoVer(repoPath, 28)
}

func migration027_listingFilePath(datadir string, slug string) string {
	return path.Join(datadir, "root", "listings", slug+".json")
}

func (Migration027) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return writeRepoVer(repoPath, 27)
}
