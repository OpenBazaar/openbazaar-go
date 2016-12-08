package api

import "testing"

func TestSettings(t *testing.T) {
	// Create, Read, Update, Patch
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/settings", settingsJSON, 200, "{}"},
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
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/settings", settingsMalformedJSON, 400, settingsMalformedJSONResponse},
	})

	// Invalid JSON
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/settings", settingsJSON, 200, "{}"},
		{"GET", "/ob/settings", "", 200, settingsJSON},
		{"PUT", "/ob/settings", settingsMalformedJSON, 400, settingsMalformedJSONResponse},
	})
}

func TestProfile(t *testing.T) {
	// Create, Update
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/profile", profileJSON, 200, profileJSON},
		{"POST", "/ob/profile", profileJSON, 409, AlreadyExistsUsePUTJSON("Profile")},
		{"PUT", "/ob/profile", profileUpdateJSON, 200, profileUpdatedJSON},
		{"PUT", "/ob/profile", profileUpdatedJSON, 200, profileUpdatedJSON},
	})
}

func TestAvatar(t *testing.T) {
	// Setting an avatar fails if we don't have a profile
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/avatar", avatarValidJSON, 500, anyResponseJSON},
	})

	// It succeeds if we have a profile and the image data is valid
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/profile", profileJSON, 200, profileJSON},
		{"POST", "/ob/avatar", avatarValidJSON, 200, avatarValidJSONResponse},
	})

	// Test invalid image data
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/profile", profileJSON, 200, profileJSON},
		{"POST", "/ob/avatar", avatarUnexpectedEOFJSON, 500, avatarUnexpectedEOFJSONResponse},
	})

	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/profile", profileJSON, 200, profileJSON},
		{"POST", "/ob/avatar", avatarInvalidTQJSON, 500, avatarInvalidTQJSONResponse},
	})
}

func TestImages(t *testing.T) {
	// Valid image
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/images", imageValidJSON, 200, imageValidJSONResponse},
	})
}

func TestHeader(t *testing.T) {
	// Setting an header fails if we don't have a profile
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/header", headerValidJSON, 500, anyResponseJSON},
	})

	// It succeeds if we have a profile and the image data is valid
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/profile", profileJSON, 200, profileJSON},
		{"POST", "/ob/header", headerValidJSON, 200, headerValidJSONResponse},
	})
}

func TestModerator(t *testing.T) {
	// Fails without profile
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/moderator", moderatorValidJSON, 500, anyResponseJSON},
	})

	// Works without profile
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/ob/profile", profileJSON, 200, profileJSON},
		{"POST", "/ob/moderator", moderatorValidJSON, 200, `{}`},

		// Recreation fails
		{"POST", "/ob/moderator", moderatorValidJSON, 409, anyResponseJSON},

		// Update
		{"PUT", "/ob/moderator", moderatorUpdatedValidJSON, 200, `{}`},
		{"DELETE", "/ob/moderator", "", 200, `{}`},
	})
}

func TestListings(t *testing.T) {
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"GET", "/ob/listings", "", 200, `[]`},
		{"GET", "/ob/inventory", "", 200, `[]`},

		// Invalid creates
		{"POST", "/ob/listing", `{`, 400, jsonUnexpectedEOF},

		{"GET", "/ob/listings", "", 200, `[]`},
		{"GET", "/ob/inventory", "", 200, `[]`},

		// TOOD: Add support for improved JSON matching to since contracts
		// change each test run due to signatures

		// Create/Get
		{"GET", "/ob/listing/ron_swanson_tshirt", "", 404, NotFoundJSON("Listing")},
		{"POST", "/ob/listing", listingJSON, 200, listingJSONResponse},
		{"GET", "/ob/listing/ron_swanson_tshirt", "", 200, anyResponseJSON},
		{"POST", "/ob/listing", listingUpdateJSON, 409, AlreadyExistsUsePUTJSON("Listing")},

		// TOOD: Add support for improved JSON matching to since contracts
		// change each test run due to signatures
		{"GET", "/ob/listings", "", 200, anyResponseJSON},

		// TODO: This returns `inventoryJSONResponse` but slices are unordered
		// so they don't get considered equal. Figure out a way to fix that.
		{"GET", "/ob/inventory", "", 200, anyResponseJSON},

		// Update inventory
		{"POST", "/ob/inventory", inventoryUpdateJSON, 200, `{}`},

		// Update/Get Listing
		{"PUT", "/ob/listing", listingUpdateJSON, 200, `{}`},
		{"GET", "/ob/listing/ron_swanson_tshirt", "", 200, anyResponseJSON},

		// Delete/Get
		{"DELETE", "/ob/listing", listingJSONResponse, 200, `{}`},
		{"DELETE", "/ob/listing", listingJSONResponse, 404, NotFoundJSON("Listing")},
		{"GET", "/ob/listing/ron_swanson_tshirt", "", 404, NotFoundJSON("Listing")},

		// Mutate non-existing listings
		{"PUT", "/ob/listing", listingUpdateJSON, 404, NotFoundJSON("Listing")},
		{"DELETE", "/ob/listing", listingJSONResponse, 404, NotFoundJSON("Listing")},
	})
}

func TestSpend(t *testing.T) {
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"POST", "/wallet/spend", spendJSON, 500, insuffientFundsJSON},
	})

	// TODO: Test on regnet with coins
}

// func TestShutdown(t *testing.T) {
// 	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
// 		{"POST", "/ob/shutdown", "", 200, `{}`},
// 	})
// }

func TestStatus(t *testing.T) {
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"GET", "/ob/status", "", 200, statusBadPeerIDJSONResponse},
	})
}

func TestWallet(t *testing.T) {
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"GET", "/ob/wallet/address", "", 200, walletAddressJSONResponse},
		{"GET", "/ob/wallet/balance", "", 200, walletBalanceJSONResponse},
		{"GET", "/ob/wallet/mnemonic", "", 200, walletMneumonicJSONResponse},
	})
}

// TODO: Need better JSON matching
// func TestConfig(t *testing.T) {
// 	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
// 		{"GET", "/ob/config", "", 200, statusBadPeerID},
// 	})
// }

// func TestPeers(t *testing.T) {
// 	// Follow, Unfollow
// 	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
// 		{"POST", "/ob/follow", `{"id":"QmRBhyTivwngraebqBVoPYCh8SBrsagqRtMwj44dMLXhwn"}`, 200, `{}`},
// 	})
// }

func Test404(t *testing.T) {
	// Test undefined endpoints
	runJSONAPIBlackboxTests(t, jsonAPIBlackboxTests{
		{"GET", "/ob/a", "{}", 404, notFoundJSON},
		{"PUT", "/ob/a", "{}", 404, notFoundJSON},
		{"POST", "/ob/a", "{}", 404, notFoundJSON},
		{"PATCH", "/ob/a", "{}", 404, notFoundJSON},
		{"DELETE", "/ob/a", "{}", 404, notFoundJSON},
	})
}
