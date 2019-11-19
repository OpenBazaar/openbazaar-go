package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"

	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	coremock "github.com/ipfs/go-ipfs/core/mock"
)

type Migration028 struct{ migrateListingsToV5Schema }

type migrateListingsToV5Schema struct{}

type (
	MigrateListingsToV5Schema_ListingThumbnail struct {
		Tiny   string `json:"tiny"`
		Small  string `json:"small"`
		Medium string `json:"medium"`
	}

	MigrateListingsToV5Schema_V5CurrencyValue struct {
		Amount   *big.Int                                       `json:"amount"`
		Currency MigrateListingsToV5Schema_V5CurrencyDefinition `json:"currency"`
	}

	MigrateListingsToV5Schema_V5CurrencyCode string

	MigrateListingsToV5Schema_V5CurrencyDefinition struct {
		Code         MigrateListingsToV5Schema_V5CurrencyCode `json:"code"`
		Divisibility uint                                     `json:"divisibility"`
	}

	MigrateListingsToV5Schema_V5ListingIndexData struct {
		Hash               string                                     `json:"hash"`
		Slug               string                                     `json:"slug"`
		Title              string                                     `json:"title"`
		Categories         []string                                   `json:"categories"`
		NSFW               bool                                       `json:"nsfw"`
		ContractType       string                                     `json:"contractType"`
		Description        string                                     `json:"description"`
		Thumbnail          MigrateListingsToV5Schema_ListingThumbnail `json:"thumbnail"`
		Price              *MigrateListingsToV5Schema_V5CurrencyValue `json:"price"`
		Modifier           float32                                    `json:"modifier"`
		ShipsTo            []string                                   `json:"shipsTo"`
		FreeShipping       []string                                   `json:"freeShipping"`
		Language           string                                     `json:"language"`
		AverageRating      float32                                    `json:"averageRating"`
		RatingCount        uint32                                     `json:"ratingCount"`
		ModeratorIDs       []string                                   `json:"moderators"`
		AcceptedCurrencies []string                                   `json:"acceptedCurrencies"`
		CryptoCurrencyCode string                                     `json:"coinType"`
	}

	MigrateListingsToV5Schema_V4price struct {
		CurrencyCode string  `json:"currencyCode"`
		Amount       uint    `json:"amount"`
		Modifier     float32 `json:"modifier"`
	}

	MigrateListingsToV5Schema_V4ListingIndexData struct {
		Hash               string                                     `json:"hash"`
		Slug               string                                     `json:"slug"`
		Title              string                                     `json:"title"`
		Categories         []string                                   `json:"categories"`
		NSFW               bool                                       `json:"nsfw"`
		ContractType       string                                     `json:"contractType"`
		Description        string                                     `json:"description"`
		Thumbnail          MigrateListingsToV5Schema_ListingThumbnail `json:"thumbnail"`
		Price              MigrateListingsToV5Schema_V4price          `json:"price"`
		ShipsTo            []string                                   `json:"shipsTo"`
		FreeShipping       []string                                   `json:"freeShipping"`
		Language           string                                     `json:"language"`
		AverageRating      float32                                    `json:"averageRating"`
		RatingCount        uint32                                     `json:"ratingCount"`
		ModeratorIDs       []string                                   `json:"moderators"`
		AcceptedCurrencies []string                                   `json:"acceptedCurrencies"`
		CryptoCurrencyCode string                                     `json:"coinType"`
	}
)

func (c *MigrateListingsToV5Schema_V5CurrencyValue) MarshalJSON() ([]byte, error) {
	var value = struct {
		Amount   string                                         `json:"amount"`
		Currency MigrateListingsToV5Schema_V5CurrencyDefinition `json:"currency"`
	}{
		Amount:   "0",
		Currency: c.Currency,
	}
	if c.Amount != nil {
		value.Amount = c.Amount.String()
	}

	return json.Marshal(value)

}

func (c *MigrateListingsToV5Schema_V5CurrencyValue) UnmarshalJSON(b []byte) error {
	var value struct {
		Amount   string                                         `json:"amount"`
		Currency MigrateListingsToV5Schema_V5CurrencyDefinition `json:"currency"`
	}
	err := json.Unmarshal(b, &value)
	if err != nil {
		return err
	}
	amt, ok := new(big.Int).SetString(value.Amount, 10)
	if !ok {
		return fmt.Errorf("invalid amount (%s)", value.Amount)
	}

	c.Amount = amt
	c.Currency = value.Currency
	return err
}

var divisibilityMap = map[string]uint{
	"BTC": 8,
	"BCH": 8,
	"LTC": 8,
	"ZEC": 8,
}

func parseV5intoV4(v5 MigrateListingsToV5Schema_V5ListingIndexData) MigrateListingsToV5Schema_V4ListingIndexData {
	if v5.ModeratorIDs == nil {
		v5.ModeratorIDs = []string{}
	}
	if v5.ShipsTo == nil {
		v5.ShipsTo = []string{}
	}
	if v5.FreeShipping == nil {
		v5.FreeShipping = []string{}
	}
	if v5.Categories == nil {
		v5.Categories = []string{}
	}
	if v5.AcceptedCurrencies == nil {
		v5.AcceptedCurrencies = []string{}
	}

	return MigrateListingsToV5Schema_V4ListingIndexData{
		Hash:         v5.Hash,
		Slug:         v5.Slug,
		Title:        v5.Title,
		Categories:   v5.Categories,
		NSFW:         v5.NSFW,
		ContractType: v5.ContractType,
		Description:  v5.Description,
		Thumbnail:    v5.Thumbnail,
		Price: MigrateListingsToV5Schema_V4price{
			CurrencyCode: string(v5.Price.Currency.Code),
			Amount:       uint(v5.Price.Amount.Uint64()),
			Modifier:     v5.Modifier,
		},
		ShipsTo:            v5.ShipsTo,
		FreeShipping:       v5.FreeShipping,
		Language:           v5.Language,
		AverageRating:      v5.AverageRating,
		RatingCount:        v5.RatingCount,
		ModeratorIDs:       v5.ModeratorIDs,
		AcceptedCurrencies: v5.AcceptedCurrencies,
		CryptoCurrencyCode: v5.CryptoCurrencyCode,
	}
}

func parseV4intoV5(v4 MigrateListingsToV5Schema_V4ListingIndexData) MigrateListingsToV5Schema_V5ListingIndexData {
	var priceValue *MigrateListingsToV5Schema_V5CurrencyValue
	divisibility, ok := divisibilityMap[strings.ToUpper(v4.Price.CurrencyCode)]
	if !ok {
		divisibility = 2
	}

	priceValue = &MigrateListingsToV5Schema_V5CurrencyValue{
		Amount: new(big.Int).SetInt64(int64(v4.Price.Amount)),
		Currency: MigrateListingsToV5Schema_V5CurrencyDefinition{
			Code:         MigrateListingsToV5Schema_V5CurrencyCode(v4.Price.CurrencyCode),
			Divisibility: divisibility,
		},
	}
	if v4.ModeratorIDs == nil {
		v4.ModeratorIDs = []string{}
	}
	if v4.ShipsTo == nil {
		v4.ShipsTo = []string{}
	}
	if v4.FreeShipping == nil {
		v4.FreeShipping = []string{}
	}
	if v4.Categories == nil {
		v4.Categories = []string{}
	}
	if v4.AcceptedCurrencies == nil {
		v4.AcceptedCurrencies = []string{}
	}

	return MigrateListingsToV5Schema_V5ListingIndexData{
		Hash:               v4.Hash,
		Slug:               v4.Slug,
		Title:              v4.Title,
		Categories:         v4.Categories,
		NSFW:               v4.NSFW,
		ContractType:       v4.ContractType,
		Description:        v4.Description,
		Thumbnail:          v4.Thumbnail,
		Modifier:           v4.Price.Modifier,
		Price:              priceValue,
		ShipsTo:            v4.ShipsTo,
		FreeShipping:       v4.FreeShipping,
		Language:           v4.Language,
		AverageRating:      v4.AverageRating,
		RatingCount:        v4.RatingCount,
		ModeratorIDs:       v4.ModeratorIDs,
		AcceptedCurrencies: v4.AcceptedCurrencies,
		CryptoCurrencyCode: v4.CryptoCurrencyCode,
	}
}

func (migrateListingsToV5Schema) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	listingsFilePath := path.Join(repoPath, "root", "listings.json")

	// Non-vendors might not have an listing.json and we don't want to error here if that's the case
	indexExists := true
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		indexExists = false
	}

	if indexExists {
		// Setup signing capabilities
		identityKey, err := migration027_GetIdentityKey(repoPath, databasePassword, testnetEnabled)
		if err != nil {
			return err
		}

		sk, err := crypto.UnmarshalPrivateKey(identityKey)
		if err != nil {
			return err
		}

		nd, err := coremock.NewMockNode()
		if err != nil {
			return err
		}

		listingHashMap := make(map[string]string)

		m := jsonpb.Marshaler{
			Indent: "    ",
		}

		var oldListingIndex []MigrateListingsToV5Schema_V4ListingIndexData
		listingsJSON, err := ioutil.ReadFile(listingsFilePath)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(listingsJSON, &oldListingIndex); err != nil {
			return err
		}

		newListingIndex := make([]MigrateListingsToV5Schema_V5ListingIndexData, len(oldListingIndex))
		for i, listing := range oldListingIndex {
			newListingIndex[i] = parseV4intoV5(listing)
		}

		for _, listing := range newListingIndex {
			listingPath := path.Join(repoPath, "root", "listings", listing.Slug+".json")
			listingBytes, err := ioutil.ReadFile(listingPath)
			if err != nil {
				return err
			}
			var signedListingJSON map[string]interface{}
			if err = json.Unmarshal(listingBytes, &signedListingJSON); err != nil {
				return err
			}

			listingJSON, listingExists := signedListingJSON["listing"]
			if !listingExists {
				continue
			}
			listing := listingJSON.(map[string]interface{})

			metadataJSON, metadataExists := listing["metadata"]
			if !metadataExists {
				continue
			}
			metadata := metadataJSON.(map[string]interface{})
			metadata["version"] = 5
			itemJSON, itemExists := listing["item"]
			if !itemExists {
				continue
			}
			item := itemJSON.(map[string]interface{})

			var (
				skus            []interface{}
				shippingOptions []interface{}
				coupons         []interface{}
			)

			skusJSON := item["skus"]
			if skusJSON != nil {
				skus = skusJSON.([]interface{})
			}
			shippingOptionsJSON := listing["shippingOptions"]
			if shippingOptionsJSON != nil {
				shippingOptions = shippingOptionsJSON.([]interface{})
			}
			couponsJSON := listing["coupons"]
			if couponsJSON != nil {
				coupons = couponsJSON.([]interface{})
			}

			pricingCurrencyJSON, pricingCurrencyExists := metadata["pricingCurrency"]
			if pricingCurrencyExists {
				pricingCurrency := pricingCurrencyJSON.(string)
				divisibility, ok := divisibilityMap[strings.ToUpper(pricingCurrency)]
				if !ok {
					divisibility = 2
				}

				item["priceCurrency"] = struct {
					Code         string `json:"code"`
					Divisibility uint32 `json:"divisibility"`
				}{
					Code:         pricingCurrency,
					Divisibility: uint32(divisibility),
				}

				delete(metadata, "pricingCurrency")
			}

			var modifier float64
			modifierJSON := metadata["priceModifier"]
			if modifierJSON != nil {
				modifier = modifierJSON.(float64)
			}

			item["priceModifier"] = modifier

			delete(metadata, "priceModifier")

			priceJSON, priceExists := item["price"]
			if priceExists {
				price := priceJSON.(float64)

				item["bigPrice"] = strconv.Itoa(int(price))

				delete(item, "price")
			}

			for _, skuJSON := range skus {
				sku := skuJSON.(map[string]interface{})

				quantityJSON, quantityExists := sku["quantity"]
				if quantityExists {
					quantity := quantityJSON.(float64)

					sku["bigQuantity"] = strconv.Itoa(int(quantity))

					delete(sku, "quantity")
				}

				surchargeJSON, surchargeExists := sku["surcharge"]
				if surchargeExists {
					surcharge := surchargeJSON.(float64)

					sku["bigSurcharge"] = strconv.Itoa(int(surcharge))

					delete(sku, "surcharge")
				}
			}

			for i, shippingOptionJSON := range shippingOptions {
				so := shippingOptionJSON.(map[string]interface{})
				var services []interface{}
				servicesJSON := so["services"]
				if servicesJSON != nil {
					services = servicesJSON.([]interface{})
				}

				for x, serviceJSON := range services {
					service := serviceJSON.(map[string]interface{})

					priceJSON := service["price"]
					price, priceExists := priceJSON.(float64)

					if priceExists {
						service["bigPrice"] = strconv.Itoa(int(price))

						delete(service, "price")
					}

					additionalItemPriceJSON, additionalPriceExists := service["additionalItemPrice"]
					if additionalPriceExists {
						additionalItemPrice := additionalItemPriceJSON.(float64)

						service["bigAdditionalItemPrice"] = strconv.Itoa(int(additionalItemPrice))

						delete(service, "additionalItemPrice")
					}

					services[x] = service
				}

				so["services"] = services
				shippingOptions[i] = so
			}

			for _, couponJSON := range coupons {
				coupon := couponJSON.(map[string]interface{})

				priceDiscountJSON, ok := coupon["priceDiscount"]
				if ok {
					priceDiscount := priceDiscountJSON.(float64)

					coupon["bigPriceDiscount"] = strconv.Itoa(int(priceDiscount))

					delete(coupon, "priceDiscount")
				}
			}

			out, err := json.MarshalIndent(signedListingJSON, "", "    ")
			if err != nil {
				return err
			}

			sl := new(pb.SignedListing)
			if err := jsonpb.UnmarshalString(string(out), sl); err != nil {
				return err
			}

			ser, err := proto.Marshal(sl.Listing)
			if err != nil {
				return err
			}

			sig, err := sk.Sign(ser)
			if err != nil {
				return err
			}

			sl.Signature = sig

			signedOut, err := m.MarshalToString(sl)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(listingPath, []byte(signedOut), os.ModePerm)
			if err != nil {
				return err
			}

			hash, err := ipfs.GetHashOfFile(nd, listingPath)
			if err != nil {
				return err
			}

			listingHashMap[sl.Listing.Slug] = hash
		}

		for i, listing := range newListingIndex {
			newListingIndex[i].Hash = listingHashMap[listing.Slug]
		}

		migratedJSON, err := json.MarshalIndent(&newListingIndex, "", "    ")
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(listingsFilePath, migratedJSON, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return writeRepoVer(repoPath, 28)
}

func (migrateListingsToV5Schema) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	listingsFilePath := path.Join(repoPath, "root", "listings.json")

	// Non-vendors might not have an listing.json and we don't want to error here if that's the case
	indexExists := true
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		indexExists = false
	}

	if indexExists {
		// Setup signing capabilities
		identityKey, err := migration027_GetIdentityKey(repoPath, databasePassword, testnetEnabled)
		if err != nil {
			return err
		}

		sk, err := crypto.UnmarshalPrivateKey(identityKey)
		if err != nil {
			return err
		}

		nd, err := coremock.NewMockNode()
		if err != nil {
			return err
		}

		listingHashMap := make(map[string]string)

		m := jsonpb.Marshaler{
			Indent: "    ",
		}

		var oldListingIndex []MigrateListingsToV5Schema_V5ListingIndexData
		listingsJSON, err := ioutil.ReadFile(listingsFilePath)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(listingsJSON, &oldListingIndex); err != nil {
			return err
		}

		newListingIndex := make([]MigrateListingsToV5Schema_V4ListingIndexData, len(oldListingIndex))
		for i, listing := range oldListingIndex {
			newListingIndex[i] = parseV5intoV4(listing)
		}

		for _, listing := range newListingIndex {
			listingPath := path.Join(repoPath, "root", "listings", listing.Slug+".json")
			listingBytes, err := ioutil.ReadFile(listingPath)
			if err != nil {
				return err
			}
			var signedListingJSON map[string]interface{}
			if err = json.Unmarshal(listingBytes, &signedListingJSON); err != nil {
				return err
			}

			listingJSON, listingExists := signedListingJSON["listing"]
			if !listingExists {
				continue
			}
			listing := listingJSON.(map[string]interface{})

			metadataJSON, metadataExists := listing["metadata"]
			if !metadataExists {
				continue
			}
			metadata := metadataJSON.(map[string]interface{})
			metadata["version"] = 4
			itemJSON, itemExists := listing["item"]
			if !itemExists {
				continue
			}
			item := itemJSON.(map[string]interface{})

			var (
				skus            []interface{}
				shippingOptions []interface{}
				coupons         []interface{}
			)

			skusJSON := item["skus"]
			if skusJSON != nil {
				skus = skusJSON.([]interface{})
			}
			shippingOptionsJSON := listing["shippingOptions"]
			if shippingOptionsJSON != nil {
				shippingOptions = shippingOptionsJSON.([]interface{})
			}
			couponsJSON := listing["coupons"]
			if couponsJSON != nil {
				coupons = couponsJSON.([]interface{})
			}

			pricingCurrencyJSON, pricingCurrencyExists := item["priceCurrency"]
			if pricingCurrencyExists {
				pricingCurrency := pricingCurrencyJSON.(map[string]interface{})

				priceCurrencyCodeJSON, currencyCodeExists := pricingCurrency["code"]
				if currencyCodeExists {
					priceCurrencyCode := priceCurrencyCodeJSON.(string)

					metadata["pricingCurrency"] = priceCurrencyCode

					delete(item, "priceCurrency")
				}
			}

			var modifier float64
			modifierJSON := item["priceModifier"]
			if modifierJSON != nil {
				modifier = modifierJSON.(float64)
			}

			metadata["priceModifier"] = modifier

			delete(item, "priceModifier")

			priceJSON, priceExists := item["bigPrice"]
			if priceExists {
				price := priceJSON.(string)

				p, ok := new(big.Int).SetString(price, 10)
				if ok {
					item["price"] = p.Uint64()
				}
				delete(item, "bigPrice")
			}

			for _, skuJSON := range skus {
				sku := skuJSON.(map[string]interface{})
				quantityJSON, quantityExists := sku["bigQuantity"]
				if quantityExists {
					quantity := quantityJSON.(string)

					p, ok := new(big.Int).SetString(quantity, 10)
					if ok {
						sku["quantity"] = p.Uint64()
					}

					delete(sku, "bigQuantity")
				}

				surchargeJSON, ok := sku["bigSurcharge"]
				if ok {
					surcharge := surchargeJSON.(string)

					s, ok := new(big.Int).SetString(surcharge, 10)
					if ok {
						sku["surcharge"] = s.Uint64()
					}

					delete(sku, "bigSurcharge")
				}
			}

			for i, shippingOptionJSON := range shippingOptions {
				so := shippingOptionJSON.(map[string]interface{})
				var services []interface{}
				servicesJSON := so["services"]
				if servicesJSON != nil {
					services = servicesJSON.([]interface{})
				}

				for x, serviceJSON := range services {
					service := serviceJSON.(map[string]interface{})

					priceJSON, priceExists := service["bigPrice"]
					if priceExists {
						price := priceJSON.(string)

						p, ok := new(big.Int).SetString(price, 10)
						if ok {
							service["price"] = p.Uint64()
						}

						delete(service, "bigPrice")
					}

					additionalItemPriceJSON, additionalPriceExists := service["bigAdditionalItemPrice"]
					if additionalPriceExists {
						additionalItemPrice := additionalItemPriceJSON.(string)

						a, ok := new(big.Int).SetString(additionalItemPrice, 10)
						if ok {
							service["additionalItemPrice"] = a.Uint64()
						}

						delete(service, "bigAdditionalItemPrice")
					}

					services[x] = service
				}

				shippingOptions[i] = so
			}

			for _, couponJSON := range coupons {
				coupon := couponJSON.(map[string]interface{})

				priceDiscountJSON, priceDiscountExists := coupon["bigPriceDiscount"]
				if priceDiscountExists {
					priceDiscount := priceDiscountJSON.(string)

					a, ok := new(big.Int).SetString(priceDiscount, 10)
					if ok {
						coupon["priceDiscount"] = a.Uint64()
					}

					delete(coupon, "bigPriceDiscount")
				}
			}

			out, err := json.MarshalIndent(signedListingJSON, "", "    ")
			if err != nil {
				return err
			}

			sl := new(pb.SignedListing)
			if err := jsonpb.UnmarshalString(string(out), sl); err != nil {
				return err
			}

			ser, err := proto.Marshal(sl.Listing)
			if err != nil {
				return err
			}

			sig, err := sk.Sign(ser)
			if err != nil {
				return err
			}

			sl.Signature = sig

			signedOut, err := m.MarshalToString(sl)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(listingPath, []byte(signedOut), os.ModePerm)
			if err != nil {
				return err
			}

			hash, err := ipfs.GetHashOfFile(nd, listingPath)
			if err != nil {
				return err
			}

			listingHashMap[sl.Listing.Slug] = hash
		}

		for i, listing := range newListingIndex {
			newListingIndex[i].Hash = listingHashMap[listing.Slug]
		}

		migratedJSON, err := json.MarshalIndent(&newListingIndex, "", "    ")
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(listingsFilePath, migratedJSON, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return writeRepoVer(repoPath, 27)
}

func migration027_GetIdentityKey(repoPath, databasePassword string, testnetEnabled bool) ([]byte, error) {
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
