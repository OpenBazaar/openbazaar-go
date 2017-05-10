package repo

import (
	"time"
)

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
	MisPaymentBuffer   *float32           `json:"mispaymentBuffer"`
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

type Coupon struct {
	Slug string
	Code string
	Hash string
}

type ChatMessage struct {
	MessageId string    `json:"messageId"`
	PeerId    string    `json:"peerId"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	Outgoing  bool      `json:"outgoing"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatConversation struct {
	PeerId    string    `json:"peerId"`
	Unread    int       `json:"unread"`
	Last      string    `json:"lastMessage"`
	Timestamp time.Time `json:"timestamp"`
	Outgoing  bool      `json:"outgoing"`
}

type Metadata struct {
	Txid       string
	Address    string
	Memo       string
	OrderId    string
	Thumbnail  string
	CanBumpFee bool
}

type Purchase struct {
	OrderId            string    `json:"orderId"`
	Slug               string    `json:"slug"`
	Timestamp          time.Time `json:"timestamp"`
	Title              string    `json:"title"`
	Thumbnail          string    `json:"thumbnail"`
	Total              uint64    `json:"total"`
	VendorId           string    `json:"vendorId"`
	VendorHandle       string    `json:"vendorHandle"`
	ShippingName       string    `json:"shippingName"`
	ShippingAddress    string    `json:"shippingAddress"`
	State              string    `json:"state"`
	Read               bool      `json:"read"`
	UnreadChatMessages int       `json:"unreadChatMessages"`
}

type Sale struct {
	OrderId            string    `json:"orderId"`
	Slug               string    `json:"slug"`
	Timestamp          time.Time `json:"timestamp"`
	Title              string    `json:"title"`
	Thumbnail          string    `json:"thumbnail"`
	Total              uint64    `json:"total"`
	BuyerId            string    `json:"buyerId"`
	BuyerHandle        string    `json:"buyerHandle"`
	ShippingName       string    `json:"shippingName"`
	ShippingAddress    string    `json:"shippingAddress"`
	State              string    `json:"state"`
	Read               bool      `json:"read"`
	UnreadChatMessages int       `json:"unreadChatMessages"`
}

type Case struct {
	CaseId             string    `json:"caseId"`
	Slug               string    `json:"slug"`
	Timestamp          time.Time `json:"timestamp"`
	Title              string    `json:"title"`
	Thumbnail          string    `json:"thumbnail"`
	Total              uint64    `json:"total"`
	BuyerId            string    `json:"buyerId"`
	BuyerHandle        string    `json:"buyerHandle"`
	VendorId           string    `json:"vendorId"`
	VendorHandle       string    `json:"vendorHandle"`
	BuyerOpened        bool      `json:"buyerOpened"`
	State              string    `json:"state"`
	Read               bool      `json:"read"`
	UnreadChatMessages int       `json:"unreadChatMessages"`
}
