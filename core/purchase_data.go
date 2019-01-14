package core

// Option represents the selected option of an item's varient
type Option struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ShippingOption represents an available method of shipment
type ShippingOption struct {
	Name    string `json:"name"`
	Service string `json:"service"`
}

// Item represents a purchaseable subject
type Item struct {
	ListingHash    string         `json:"listingHash"`
	Quantity       uint64         `json:"quantity"`
	Options        []Option       `json:"options"`
	Shipping       ShippingOption `json:"shipping"`
	Memo           string         `json:"memo"`
	Coupons        []string       `json:"coupons"`
	PaymentAddress string         `json:"paymentAddress"`
}

// PurchaseData represents a request to purchase a set of Items
type PurchaseData struct {
	ShipTo               string  `json:"shipTo"`
	Address              string  `json:"address"`
	City                 string  `json:"city"`
	State                string  `json:"state"`
	PostalCode           string  `json:"postalCode"`
	CountryCode          string  `json:"countryCode"`
	AddressNotes         string  `json:"addressNotes"`
	Moderator            string  `json:"moderator"`
	Items                []Item  `json:"items"`
	AlternateContactInfo string  `json:"alternateContactInfo"`
	RefundAddress        *string `json:"refundAddress"` //optional, can be left out of json
	PaymentCoin          string  `json:"paymentCoin"`
}
