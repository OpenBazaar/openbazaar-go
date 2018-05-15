package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestMain(m *testing.M) {
	// Create a test server
	gateway, err := newTestGateway()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err = gateway.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Run tests
	retCode := m.Run()

	// Shutdown test server
	err = gateway.Close()
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(retCode)
}

func TestSettings(t *testing.T) {
	// Create, Read, Update, Patch
	runAPITests(t, apiTests{
		{"POST", "/ob/settings", settingsJSON, 200, settingsJSON},
		{"GET", "/ob/settings", "", 200, settingsJSON},
		{"POST", "/ob/settings", settingsJSON, 409, settingsAlreadyExistsJSON},
		{"PUT", "/ob/settings", settingsUpdateJSON, 200, "{}"},
		{"GET", "/ob/settings", "", 200, settingsUpdateJSON},
		{"PUT", "/ob/settings", settingsUpdateJSON, 200, "{}"},
		{"GET", "/ob/settings", "", 200, settingsUpdateJSON},
		{"PATCH", "/ob/settings", settingsPatchJSON, 200, "{}"},
		{"GET", "/ob/settings", "", 200, settingsPatchedJSON},
	})

	// Invalid JSON
	runAPITests(t, apiTests{
		{"POST", "/ob/settings", settingsMalformedJSON, 400, settingsMalformedJSONResponse},
	})

	// Invalid JSON
	runAPITests(t, apiTests{
		{"POST", "/ob/settings", settingsJSON, 200, settingsJSON},
		{"GET", "/ob/settings", "", 200, settingsJSON},
		{"PUT", "/ob/settings", settingsMalformedJSON, 400, settingsMalformedJSONResponse},
	})
}

func TestProfile(t *testing.T) {
	// Create, Update
	runAPITests(t, apiTests{
		{"POST", "/ob/profile", profileJSON, 200, anyResponseJSON},
		{"POST", "/ob/profile", profileJSON, 409, AlreadyExistsUsePUTJSON("Profile")},
		{"PUT", "/ob/profile", profileUpdateJSON, 200, anyResponseJSON},
		{"PUT", "/ob/profile", profileUpdatedJSON, 200, anyResponseJSON},
	})
}

func TestAvatar(t *testing.T) {
	// Setting an avatar fails if we don't have a profile
	runAPITests(t, apiTests{
		{"POST", "/ob/avatar", avatarValidJSON, 500, anyResponseJSON},
	})

	// It succeeds if we have a profile and the image data is valid
	runAPITests(t, apiTests{
		{"POST", "/ob/profile", profileJSON, 200, anyResponseJSON},
		{"POST", "/ob/avatar", avatarValidJSON, 200, avatarValidJSONResponse},
	})

	// Test invalid image data
	runAPITests(t, apiTests{
		{"POST", "/ob/profile", profileJSON, 200, anyResponseJSON},
		{"POST", "/ob/avatar", avatarUnexpectedEOFJSON, 500, avatarUnexpectedEOFJSONResponse},
	})

	runAPITests(t, apiTests{
		{"POST", "/ob/profile", profileJSON, 200, anyResponseJSON},
		{"POST", "/ob/avatar", avatarInvalidTQJSON, 500, avatarInvalidTQJSONResponse},
	})
}

func TestImages(t *testing.T) {
	// Valid image
	runAPITests(t, apiTests{
		{"POST", "/ob/images", imageValidJSON, 200, imageValidJSONResponse},
	})
}

func TestHeader(t *testing.T) {
	// Setting an header fails if we don't have a profile
	runAPITests(t, apiTests{
		{"POST", "/ob/header", headerValidJSON, 500, anyResponseJSON},
	})

	// It succeeds if we have a profile and the image data is valid
	runAPITests(t, apiTests{
		{"POST", "/ob/profile", profileJSON, 200, anyResponseJSON},
		{"POST", "/ob/header", headerValidJSON, 200, headerValidJSONResponse},
	})
}

func TestModerator(t *testing.T) {
	// Fails without profile
	runAPITests(t, apiTests{
		{"PUT", "/ob/moderator", moderatorValidJSON, http.StatusConflict, anyResponseJSON},
	})

	// Works with profile
	runAPITests(t, apiTests{
		{"POST", "/ob/profile", profileJSON, 200, anyResponseJSON},

		// TODO: Enable after fixing bug that requires peers in order to set moderator status
		// {"PUT", "/ob/moderator", moderatorValidJSON, 200, `{}`},

		// // Update
		// {"PUT", "/ob/moderator", moderatorUpdatedValidJSON, 200, `{}`},
		{"DELETE", "/ob/moderator", "", 200, `{}`},
	})
}

func TestListings(t *testing.T) {
	goodListingJSON := jsonFor(t, factory.NewListing("ron-swanson-tshirt"))
	updatedListing := factory.NewListing("ron-swanson-tshirt")
	updatedListing.Taxes = []*pb.Listing_Tax{
		{
			Percentage:  17,
			TaxShipping: true,
			TaxType:     "Sales tax",
			TaxRegions:  []pb.CountryCode{pb.CountryCode_UNITED_STATES},
		},
	}
	updatedListingJSON := jsonFor(t, updatedListing)

	runAPITests(t, apiTests{
		{"GET", "/ob/listings", "", 200, `[]`},
		{"GET", "/ob/inventory", "", 200, `{}`},

		// Invalid creates
		{"POST", "/ob/listing", `{`, 400, jsonUnexpectedEOF},

		{"GET", "/ob/listings", "", 200, `[]`},
		{"GET", "/ob/inventory", "", 200, `{}`},

		// TODO: Add support for improved JSON matching to since contracts
		// change each test run due to signatures

		// Create/Get
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},
		{"POST", "/ob/listing", goodListingJSON, 200, `{"slug": "ron-swanson-tshirt"}`},
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 200, anyResponseJSON},
		{"POST", "/ob/listing", updatedListingJSON, 409, AlreadyExistsUsePUTJSON("Listing")},

		// TODO: Add support for improved JSON matching to since contracts
		// change each test run due to signatures
		{"GET", "/ob/listings", "", 200, anyResponseJSON},

		// TODO: This returns `inventoryJSONResponse` but slices are unordered
		// so they don't get considered equal. Figure out a way to fix that.
		{"GET", "/ob/inventory", "", 200, anyResponseJSON},

		// Update inventory
		{"POST", "/ob/inventory", inventoryUpdateJSON, 200, `{}`},

		// Update/Get Listing
		{"PUT", "/ob/listing", updatedListingJSON, 200, `{}`},
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 200, anyResponseJSON},

		// Delete/Get
		{"DELETE", "/ob/listing/ron-swanson-tshirt", "", 200, `{}`},
		{"DELETE", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},

		// Mutate non-existing listings
		{"PUT", "/ob/listing", updatedListingJSON, 404, NotFoundJSON("Listing")},
		{"DELETE", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},
	})
}

func TestCryptoListings(t *testing.T) {
	listing := factory.NewCryptoListing("crypto")
	updatedListing := *listing

	runAPITests(t, apiTests{
		{"POST", "/ob/listing", jsonFor(t, listing), 200, `{"slug": "crypto"}`},
		{"GET", "/ob/listing/crypto", jsonFor(t, &updatedListing), 200, anyResponseJSON},

		{"PUT", "/ob/listing", jsonFor(t, &updatedListing), 200, "{}"},
		{"PUT", "/ob/listing", jsonFor(t, &updatedListing), 200, "{}"},
		{"GET", "/ob/listing/crypto", jsonFor(t, &updatedListing), 200, anyResponseJSON},

		{"DELETE", "/ob/listing/crypto", "", 200, `{}`},
		{"DELETE", "/ob/listing/crypto", "", 404, NotFoundJSON("Listing")},
		{"GET", "/ob/listing/crypto", "", 404, NotFoundJSON("Listing")},
	})
}

func TestListingsQuantity(t *testing.T) {
	listing := factory.NewListing("crypto")
	runAPITest(t, apiTest{
		"POST", "/ob/listing", jsonFor(t, listing), 200, `{"slug": "crypto"}`,
	})

	listing.Item.Skus[0].Quantity = 0
	runAPITest(t, apiTest{
		"POST", "/ob/listing", jsonFor(t, listing), 200, anyResponseJSON,
	})

	listing.Item.Skus[0].Quantity = -1
	runAPITest(t, apiTest{
		"POST", "/ob/listing", jsonFor(t, listing), 200, anyResponseJSON,
	})
}

func TestCryptoListingsQuantity(t *testing.T) {
	listing := factory.NewCryptoListing("crypto")
	runAPITest(t, apiTest{
		"POST", "/ob/listing", jsonFor(t, listing), 200, `{"slug": "crypto"}`,
	})

	listing.Item.Skus[0].Quantity = 0
	runAPITest(t, apiTest{
		"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(core.ErrCryptocurrencySkuQuantityInvalid),
	})

	listing.Item.Skus[0].Quantity = -1
	runAPITest(t, apiTest{
		"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(core.ErrCryptocurrencySkuQuantityInvalid),
	})
}

func TestCryptoListingsNoCoinType(t *testing.T) {
	listing := factory.NewCryptoListing("crypto")
	listing.Metadata.CoinType = ""

	runAPITests(t, apiTests{
		{"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(core.ErrCryptocurrencyListingCoinTypeRequired)},
	})
}

func TestCryptoListingsCoinDivisibilityIncorrect(t *testing.T) {
	listing := factory.NewCryptoListing("crypto")
	runAPITests(t, apiTests{
		{"POST", "/ob/listing", jsonFor(t, listing), 200, anyResponseJSON},
	})

	listing.Metadata.CoinDivisibility = 1e7
	runAPITests(t, apiTests{
		{"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(core.ErrListingCoinDivisibilityIncorrect)},
	})

	listing.Metadata.CoinDivisibility = 0
	runAPITests(t, apiTests{
		{"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(core.ErrListingCoinDivisibilityIncorrect)},
	})
}

func TestCryptoListingsIllegalFields(t *testing.T) {
	runTest := func(listing *pb.Listing, err error) {
		runAPITests(t, apiTests{
			{"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(err)},
		})
	}

	physicalListing := factory.NewListing("physical")

	listing := factory.NewCryptoListing("crypto")
	listing.Metadata.PricingCurrency = "btc"
	runTest(listing, core.ErrCryptocurrencyListingIllegalField("metadata.pricingCurrency"))

	listing = factory.NewCryptoListing("crypto")
	listing.Item.Condition = "new"
	runTest(listing, core.ErrCryptocurrencyListingIllegalField("item.condition"))

	listing = factory.NewCryptoListing("crypto")
	listing.Item.Options = physicalListing.Item.Options
	runTest(listing, core.ErrCryptocurrencyListingIllegalField("item.options"))

	listing = factory.NewCryptoListing("crypto")
	listing.ShippingOptions = physicalListing.ShippingOptions
	runTest(listing, core.ErrCryptocurrencyListingIllegalField("shippingOptions"))

	listing = factory.NewCryptoListing("crypto")
	listing.Coupons = physicalListing.Coupons
	runTest(listing, core.ErrCryptocurrencyListingIllegalField("coupons"))
}

func TestMarketRatePrice(t *testing.T) {
	listing := factory.NewListing("listing")
	listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
	listing.Item.Price = 1

	runAPITests(t, apiTests{
		{"POST", "/ob/listing", jsonFor(t, listing), 500, errorResponseJSON(core.ErrMarketPriceListingIllegalField("item.price"))},
	})
}

func TestStatus(t *testing.T) {
	runAPITests(t, apiTests{
		{"GET", "/ob/status", "", 400, anyResponseJSON},
		{"GET", "/ob/status/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "", 200, anyResponseJSON},
	})
}

func TestWallet(t *testing.T) {
	runAPITests(t, apiTests{
		{"GET", "/wallet/address", "", 200, walletAddressJSONResponse},
		{"GET", "/wallet/balance", "", 200, walletBalanceJSONResponse},
		{"GET", "/wallet/mnemonic", "", 200, walletMneumonicJSONResponse},
		{"POST", "/wallet/spend", spendJSON, 400, insuffientFundsJSON},
		// TODO: Test successful spend on regnet with coins
	})
}

func TestConfig(t *testing.T) {
	runAPITests(t, apiTests{
		// TODO: Need better JSON matching
		{"GET", "/ob/config", "", 200, anyResponseJSON},
	})
}

func Test404(t *testing.T) {
	// Test undefined endpoints
	runAPITests(t, apiTests{
		{"GET", "/ob/a", "{}", 404, notFoundJSON},
		{"PUT", "/ob/a", "{}", 404, notFoundJSON},
		{"POST", "/ob/a", "{}", 404, notFoundJSON},
		{"PATCH", "/ob/a", "{}", 404, notFoundJSON},
		{"DELETE", "/ob/a", "{}", 404, notFoundJSON},
	})
}

func TestPosts(t *testing.T) {
	runAPITests(t, apiTests{
		{"GET", "/ob/posts", "", 200, `[]`},

		// Invalid creates
		{"POST", "/ob/post", `{`, 400, jsonUnexpectedEOF},

		{"GET", "/ob/posts", "", 200, `[]`},

		// Create/Get
		{"GET", "/ob/post/test1", "", 404, NotFoundJSON("Post")},
		{"POST", "/ob/post", postJSON, 200, postJSONResponse},
		{"GET", "/ob/post/test1", "", 200, anyResponseJSON},
		{"POST", "/ob/post", postUpdateJSON, 409, AlreadyExistsUsePUTJSON("Post")},

		{"GET", "/ob/posts", "", 200, anyResponseJSON},

		// Update/Get Post
		{"PUT", "/ob/post", postUpdateJSON, 200, `{}`},
		{"GET", "/ob/post/test1", "", 200, anyResponseJSON},

		// Delete/Get
		{"DELETE", "/ob/post/test1", "", 200, `{}`},
		{"DELETE", "/ob/post/test1", "", 404, NotFoundJSON("Post")},
		{"GET", "/ob/post/test1", "", 404, NotFoundJSON("Post")},

		// Mutate non-existing listings
		{"PUT", "/ob/post", postUpdateJSON, 404, NotFoundJSON("Post")},
		{"DELETE", "/ob/post/test1", "", 404, NotFoundJSON("Post")},
	})
}
