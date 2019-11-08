package factory

import (
	"encoding/json"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func MustNewValidSettings() repo.SettingsData {
	var (
		validSettings = `{
		"paymentDataInQR": true,
		"showNotifications": true,
		"showNsfw": true,
		"shippingAddresses": [],
		"localCurrency": "USD",
		"country": "United State of Shipping",
		"termsAndConditions": "Terms and Conditions",
		"refundPolicy": "Refund policy.",
		"blockedNodes": [],
		"storeModerators": [],
		"mispaymentBuffer": 1,
		"smtpSettings"  : {
			"notifications": false,
			"password": "",
			"recipientEmail": "",
			"senderEmail": "",
			"serverAddress": "",
			"username": ""
		},
		"version": "",
		"preferredCurrencies": ["BTC", "BCH"]
	}`
		settings repo.SettingsData
	)
	if err := json.Unmarshal([]byte(validSettings), &settings); err != nil {
		panic(err)
	}
	settings.ShippingAddresses = &[]repo.ShippingAddress{
		NewValidShippingAddress(),
	}
	return settings
}

func NewValidShippingAddress() repo.ShippingAddress {
	return repo.ShippingAddress{
		Name:           "Shipping Name",
		Company:        "Shipping Company",
		AddressLineOne: "123 Address Street",
		AddressLineTwo: "Suite H",
		City:           "Shipping City",
		State:          "Shipping State",
		Country:        "United States of Shipping",
		PostalCode:     "12345-6789",
		AddressNotes:   "This is a fake yet valid address for testing.",
	}
}
