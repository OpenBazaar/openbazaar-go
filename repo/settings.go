package repo

type SettingsData struct {
	PaymentDataInQR    *bool              `json:"paymentDataInQR"`
	ShowNotifications  *bool              `json:"showNotificatons"`
	ShowNsfw           *bool              `json:"showNsfw"`
	ShippingAddresses  *[]ShippingAddress `json:"shippingAddresses"`
	LocalCurrency      *string            `json:"localCurrency"`
	Country            *string            `json:"country"`
	Language           *string            `json:"language"`
	TermsAndConditions *string            `json:"termsAndConditions"`
	RefundPolicy       *string            `json:"refundPolicy"`
	BlockedNodes       *[]string          `json:"blockedNodes"`
	StoreModerators    *[]string          `json:"storeModerators"`
	SMTPSettings       *SMTPSettings      `json:"smtpSettings"`
}

type ShippingAddress struct {
	Name           string
	Company        string
	AddressLineOne string
	AddressLineTwo string
	City           string
	State          string
	Country        string
	PostalCode     string
}

type SMTPSettings struct {
	Notifications  bool
	ServerAddress  string
	Username       string
	Password       string
	SenderEmail    string
	RecipientEmail string
}
