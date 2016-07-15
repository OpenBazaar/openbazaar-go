package repo

type SettingsData struct {
	PaymentDataInQR    *bool
	ShowNotifications  *bool
	ShowNsfw           *bool
	ShippingAddresses  *[]ShippingAddress
	LocalCurrency      *string
	Country            *string
	Language           *string
	TermsAndConditions *string
	RefundPolicy       *string
	BlockedNodes       *[]string
	StoreModerators    *[]string
	SMTPSettings       *SMTPSettings
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