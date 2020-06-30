package repo

type CheckoutVariant struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Price string `json:"price"`
}

type CheckoutCurrency struct {
	Code         string `json:"code"`
	Divisibility int    `json:"divisibility"`
}

type CheckoutBreakdown struct {
	BasePrice       string `json:"basePrice"`
	Coupon          string `json:"coupon"`
	OptionSurcharge string `json:"optionSurcharge"`
	Quantity        string `json:"quantity"`
	ShippingPrice   string `json:"shippingPrice"`
	Tax             string `json:"tax"`
	TotalPrice      string `json:"totalPrice"`
}
