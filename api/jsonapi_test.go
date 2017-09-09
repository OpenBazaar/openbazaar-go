package api

import (
	"net/http"
	"os"
	"testing"
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
	runAPITests(t, apiTests{
		{"GET", "/ob/listings", "", 200, `[]`},
		{"GET", "/ob/inventory", "", 200, `[]`},

		// Invalid creates
		{"POST", "/ob/listing", `{`, 400, jsonUnexpectedEOF},

		{"GET", "/ob/listings", "", 200, `[]`},
		{"GET", "/ob/inventory", "", 200, `[]`},

		// TODO: Add support for improved JSON matching to since contracts
		// change each test run due to signatures

		// Create/Get
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},
		{"POST", "/ob/listing", listingJSON, 200, listingJSONResponse},
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 200, anyResponseJSON},
		{"POST", "/ob/listing", listingUpdateJSON, 409, AlreadyExistsUsePUTJSON("Listing")},

		// TODO: Add support for improved JSON matching to since contracts
		// change each test run due to signatures
		{"GET", "/ob/listings", "", 200, anyResponseJSON},

		// TODO: This returns `inventoryJSONResponse` but slices are unordered
		// so they don't get considered equal. Figure out a way to fix that.
		{"GET", "/ob/inventory", "", 200, anyResponseJSON},

		// Update inventory
		{"POST", "/ob/inventory", inventoryUpdateJSON, 200, `{}`},

		// Update/Get Listing
		{"PUT", "/ob/listing", listingUpdateJSON, 200, `{}`},
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 200, anyResponseJSON},

		// Delete/Get
		{"DELETE", "/ob/listing/ron-swanson-tshirt", "", 200, `{}`},
		{"DELETE", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},
		{"GET", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},

		// Mutate non-existing listings
		{"PUT", "/ob/listing", listingUpdateJSON, 404, NotFoundJSON("Listing")},
		{"DELETE", "/ob/listing/ron-swanson-tshirt", "", 404, NotFoundJSON("Listing")},
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
		{"POST", "/wallet/spend", spendJSON, 500, insuffientFundsJSON},
		// TODO: Test successful spend on regnet with coins
	})
}

func TestConfig(t *testing.T) {
	runAPITests(t, apiTests{
		// TODO: Need better JSON matching
		{"GET", "/ob/config", "", 200, anyResponseJSON},
	})
}

func TestPeers(t *testing.T) {
	// Follow, Unfollow
	runAPITests(t, apiTests{
		{"POST", "/ob/follow", `{"id":"QmRBhyTivwngraebqBVoPYCh8SBrsagqRtMwj44dMLXhwn"}`, 500, peerNotFoundInTableJSON},
		// {"POST", "/ob/follow", `{"id":"QmRBhyTivwngraebqBVoPYCh8SBrsagqRtMwj44dMLXhwn"}`, 200, `{}`},
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
