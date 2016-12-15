package repo

type SettingsData struct {
	PaymentDataInQR    *bool              `json:"paymentDataInQR"`
	ShowNotifications  *bool              `json:"showNotifications"`
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
	Version            *string            `json:"version"`
}

type ShippingAddress struct {
	Name           string `json:"name"`
	Company        string `json:"company"`
	AddressLineOne string `json:"addressLineOne"`
	AddressLineTwo string `json:"addressLineTwo"`
	City           string `json:"city"`
	State          string `json:"state"`
	Country        string `json:"country"`
	PostalCode     string `json:"postalCode"`
	AddressNotes   string `json:"addressNotes"`
}

type SMTPSettings struct {
	Notifications  bool   `json:"notifications"`
	ServerAddress  string `json:"serverAddress"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	SenderEmail    string `json:"senderEmail"`
	RecipientEmail string `json:"recipientEmail"`
}
