package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"strings"
	"time"

	"bytes"
	"strconv"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	ipfspath "github.com/ipfs/go-ipfs/path"
)

type option struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type shippingOption struct {
	Name    string `json:"name"`
	Service string `json:"service"`
}

type item struct {
	ListingHash    string         `json:"listingHash"`
	Quantity       uint64         `json:"quantity"`
	Options        []option       `json:"options"`
	Shipping       shippingOption `json:"shipping"`
	Memo           string         `json:"memo"`
	Coupons        []string       `json:"coupons"`
	PaymentAddress string         `json:"paymentAddress"`
}

type PurchaseData struct {
	ShipTo               string  `json:"shipTo"`
	Address              string  `json:"address"`
	City                 string  `json:"city"`
	State                string  `json:"state"`
	PostalCode           string  `json:"postalCode"`
	CountryCode          string  `json:"countryCode"`
	AddressNotes         string  `json:"addressNotes"`
	Moderator            string  `json:"moderator"`
	Items                []item  `json:"items"`
	AlternateContactInfo string  `json:"alternateContactInfo"`
	RefundAddress        *string `json:"refundAddress"` //optional, can be left out of json
}

const (
	// We use this to check to see if the approximate fee to release funds from escrow is greater than 1/4th of the amount
	// being released. If so, we prevent the purchase from being made as it severely cuts into the vendor's profits.
	// TODO: this probably should not be hardcoded but making it adaptive requires all wallet implementations to provide this data.
	// TODO: for now, this is probably OK as it's just an approximation.
	EscrowReleaseSize                             = 337
	CryptocurrencyPurchasePaymentAddressMaxLength = 512
)

func (n *OpenBazaarNode) Purchase(data *PurchaseData) (orderId string, paymentAddress string, paymentAmount uint64, vendorOnline bool, err error) {
	contract, err := n.createContractWithOrder(data)
	if err != nil {
		return "", "", 0, false, err
	}

	// Add payment data and send to vendor
	if data.Moderator != "" { // Moderated payment
		if data.Moderator == n.IpfsNode.Identity.Pretty() {
			return "", "", 0, false, errors.New("Cannot select self as moderator")
		}
		if data.Moderator == contract.VendorListings[0].VendorID.PeerID {
			return "", "", 0, false, errors.New("Cannot select vendor as moderator")
		}
		payment := new(pb.Order_Payment)
		payment.Method = pb.Order_Payment_MODERATED
		payment.Moderator = data.Moderator

		ipnsPath := ipfspath.FromString(data.Moderator + "/profile.json")
		profileBytes, err := n.IPNSResolveThenCat(ipnsPath, time.Minute)
		if err != nil {
			return "", "", 0, false, errors.New("Moderator could not be found")
		}
		profile := new(pb.Profile)
		err = jsonpb.UnmarshalString(string(profileBytes), profile)
		if err != nil {
			return "", "", 0, false, err
		}
		moderatorKeyBytes, err := hex.DecodeString(profile.BitcoinPubkey)
		if err != nil {
			return "", "", 0, false, err
		}
		if !profile.Moderator || profile.ModeratorInfo == nil || len(profile.ModeratorInfo.AcceptedCurrencies) == 0 {
			return "", "", 0, false, errors.New("Moderator is not capable of moderating this transaction")
		}
		currencyAccepted := false
		for _, currency := range profile.ModeratorInfo.AcceptedCurrencies {
			if strings.ToLower(currency) == strings.ToLower(n.Wallet.CurrencyCode()) {
				currencyAccepted = true
			}
		}
		if !currencyAccepted {
			return "", "", 0, false, errors.New("Moderator does not accept our currency")
		}
		total, err := n.CalculateOrderTotal(contract)
		if err != nil {
			return "", "", 0, false, err
		}
		payment.Amount = total
		fpb := n.Wallet.GetFeePerByte(wallet.NORMAL)
		if (fpb * EscrowReleaseSize) > (payment.Amount / 4) {
			return "", "", 0, false, errors.New("Transaction fee too high for moderated payment")
		}

		/* Generate a payment address using the first child key derived from the buyers's,
		   vendors's and moderator's masterPubKey and a random chaincode. */
		chaincode := make([]byte, 32)
		_, err = rand.Read(chaincode)
		if err != nil {
			return "", "", 0, false, err
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
		hdKey := hd.NewExtendedKey(
			n.Wallet.Params().HDPublicKeyID[:],
			contract.VendorListings[0].VendorID.Pubkeys.Bitcoin,
			chaincode,
			parentFP,
			0,
			0,
			false)

		vendorKey, err := hdKey.Child(0)
		if err != nil {
			return "", "", 0, false, err
		}
		hdKey = hd.NewExtendedKey(
			n.Wallet.Params().HDPublicKeyID[:],
			contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
			chaincode,
			parentFP,
			0,
			0,
			false)

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			return "", "", 0, false, err
		}
		hdKey = hd.NewExtendedKey(
			n.Wallet.Params().HDPublicKeyID[:],
			moderatorKeyBytes,
			chaincode,
			parentFP,
			0,
			0,
			false)

		moderatorKey, err := hdKey.Child(0)
		if err != nil {
			return "", "", 0, false, err
		}
		modPub, err := moderatorKey.ECPubKey()
		if err != nil {
			return "", "", 0, false, err
		}
		payment.ModeratorKey = modPub.SerializeCompressed()

		timeout, err := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2, timeout, vendorKey)
		if err != nil {
			return "", "", 0, false, err
		}
		payment.Address = addr.EncodeAddress()
		payment.RedeemScript = hex.EncodeToString(redeemScript)
		payment.Chaincode = hex.EncodeToString(chaincode)
		contract.BuyerOrder.Payment = payment
		contract.BuyerOrder.RefundFee = n.Wallet.GetFeePerByte(wallet.NORMAL)

		script, err := n.Wallet.AddressToScript(addr)
		if err != nil {
			return "", "", 0, false, err
		}
		err = n.Wallet.AddWatchedScript(script)
		if err != nil {
			return "", "", 0, false, err
		}

		contract, err = n.SignOrder(contract)
		if err != nil {
			return "", "", 0, false, err
		}

		// Send to order vendor
		resp, err := n.SendOrder(contract.VendorListings[0].VendorID.PeerID, contract)
		if err != nil { // Vendor offline
			// Send using offline messaging
			log.Warningf("Vendor %s is offline, sending offline order message", contract.VendorListings[0].VendorID.PeerID)
			peerId, err := peer.IDB58Decode(contract.VendorListings[0].VendorID.PeerID)
			if err != nil {
				return "", "", 0, false, err
			}
			any, err := ptypes.MarshalAny(contract)
			if err != nil {
				return "", "", 0, false, err
			}
			m := pb.Message{
				MessageType: pb.Message_ORDER,
				Payload:     any,
			}
			k, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.SendOfflineMessage(peerId, &k, &m)
			if err != nil {
				return "", "", 0, false, err
			}
			orderId, err := n.CalcOrderId(contract.BuyerOrder)
			if err != nil {
				return "", "", 0, false, err
			}
			n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
			return orderId, contract.BuyerOrder.Payment.Address, contract.BuyerOrder.Payment.Amount, false, err
		} else { // Vendor responded
			if resp.MessageType == pb.Message_ERROR {
				return "", "", 0, false, extractErrorMessage(resp)
			}
			if resp.MessageType != pb.Message_ORDER_CONFIRMATION {
				return "", "", 0, false, errors.New("Vendor responded to the order with an incorrect message type")
			}
			rc := new(pb.RicardianContract)
			err := proto.Unmarshal(resp.Payload.Value, rc)
			if err != nil {
				return "", "", 0, false, errors.New("Error parsing the vendor's response")
			}
			contract.VendorOrderConfirmation = rc.VendorOrderConfirmation
			for _, sig := range rc.Signatures {
				if sig.Section == pb.Signature_ORDER_CONFIRMATION {
					contract.Signatures = append(contract.Signatures, sig)
				}
			}
			err = n.ValidateOrderConfirmation(contract, true)
			if err != nil {
				return "", "", 0, false, err
			}
			if contract.VendorOrderConfirmation.PaymentAddress != contract.BuyerOrder.Payment.Address {
				return "", "", 0, false, errors.New("Vendor responded with incorrect multisig address")
			}
			orderId, err := n.CalcOrderId(contract.BuyerOrder)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
			if err != nil {
				return "", "", 0, false, err
			}
			return orderId, contract.VendorOrderConfirmation.PaymentAddress, contract.BuyerOrder.Payment.Amount, true, nil
		}
	} else { // Direct payment
		payment := new(pb.Order_Payment)
		payment.Method = pb.Order_Payment_ADDRESS_REQUEST
		total, err := n.CalculateOrderTotal(contract)
		if err != nil {
			return "", "", 0, false, err
		}

		payment.Amount = total
		contract.BuyerOrder.Payment = payment
		contract, err = n.SignOrder(contract)
		if err != nil {
			return "", "", 0, false, err
		}

		// Send to order vendor and request a payment address
		resp, err := n.SendOrder(contract.VendorListings[0].VendorID.PeerID, contract)
		if err != nil { // Vendor offline
			// Change payment code to direct

			fpb := n.Wallet.GetFeePerByte(wallet.NORMAL)
			if (fpb * EscrowReleaseSize) > (payment.Amount / 4) {
				return "", "", 0, false, errors.New("Transaction fee too high for offline 2of2 multisig payment")
			}
			payment.Method = pb.Order_Payment_DIRECT

			/* Generate a payment address using the first child key derived from the buyer's
			   and vendors's masterPubKeys and a random chaincode. */
			chaincode := make([]byte, 32)
			_, err := rand.Read(chaincode)
			if err != nil {
				return "", "", 0, false, err
			}
			parentFP := []byte{0x00, 0x00, 0x00, 0x00}
			hdKey := hd.NewExtendedKey(
				n.Wallet.Params().HDPublicKeyID[:],
				contract.VendorListings[0].VendorID.Pubkeys.Bitcoin,
				chaincode,
				parentFP,
				0,
				0,
				false)

			vendorKey, err := hdKey.Child(0)
			if err != nil {
				return "", "", 0, false, err
			}
			hdKey = hd.NewExtendedKey(
				n.Wallet.Params().HDPublicKeyID[:],
				contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
				chaincode,
				parentFP,
				0,
				0,
				false)

			buyerKey, err := hdKey.Child(0)
			if err != nil {
				return "", "", 0, false, err
			}
			addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey}, 1, time.Duration(0), nil)
			if err != nil {
				return "", "", 0, false, err
			}
			payment.Address = addr.EncodeAddress()
			payment.RedeemScript = hex.EncodeToString(redeemScript)
			payment.Chaincode = hex.EncodeToString(chaincode)

			script, err := n.Wallet.AddressToScript(addr)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.Wallet.AddWatchedScript(script)
			if err != nil {
				return "", "", 0, false, err
			}

			// Remove signature and resign
			contract.Signatures = []*pb.Signature{contract.Signatures[0]}
			contract, err = n.SignOrder(contract)
			if err != nil {
				return "", "", 0, false, err
			}

			// Send using offline messaging
			log.Warningf("Vendor %s is offline, sending offline order message", contract.VendorListings[0].VendorID.PeerID)
			peerId, err := peer.IDB58Decode(contract.VendorListings[0].VendorID.PeerID)
			if err != nil {
				return "", "", 0, false, err
			}
			any, err := ptypes.MarshalAny(contract)
			if err != nil {
				return "", "", 0, false, err
			}
			m := pb.Message{
				MessageType: pb.Message_ORDER,
				Payload:     any,
			}
			k, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.SendOfflineMessage(peerId, &k, &m)
			if err != nil {
				return "", "", 0, false, err
			}
			orderId, err := n.CalcOrderId(contract.BuyerOrder)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
			if err != nil {
				return "", "", 0, false, err
			}
			return orderId, contract.BuyerOrder.Payment.Address, contract.BuyerOrder.Payment.Amount, false, err
		} else { // Vendor responded
			if resp.MessageType == pb.Message_ERROR {
				return "", "", 0, false, extractErrorMessage(resp)
			}
			if resp.MessageType != pb.Message_ORDER_CONFIRMATION {
				return "", "", 0, false, errors.New("Vendor responded to the order with an incorrect message type")
			}
			if resp.Payload == nil {
				return "", "", 0, false, errors.New("Vendor responded with nil payload")
			}
			rc := new(pb.RicardianContract)
			err := proto.Unmarshal(resp.Payload.Value, rc)
			if err != nil {
				return "", "", 0, false, errors.New("Error parsing the vendor's response")
			}
			contract.VendorOrderConfirmation = rc.VendorOrderConfirmation
			for _, sig := range rc.Signatures {
				if sig.Section == pb.Signature_ORDER_CONFIRMATION {
					contract.Signatures = append(contract.Signatures, sig)
				}
			}
			err = n.ValidateOrderConfirmation(contract, true)
			if err != nil {
				return "", "", 0, false, err
			}
			addr, err := n.Wallet.DecodeAddress(contract.VendorOrderConfirmation.PaymentAddress)
			if err != nil {
				return "", "", 0, false, err
			}
			script, err := n.Wallet.AddressToScript(addr)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.Wallet.AddWatchedScript(script)
			if err != nil {
				return "", "", 0, false, err
			}
			orderId, err := n.CalcOrderId(contract.BuyerOrder)
			if err != nil {
				return "", "", 0, false, err
			}
			err = n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
			if err != nil {
				return "", "", 0, false, err
			}
			return orderId, contract.VendorOrderConfirmation.PaymentAddress, contract.BuyerOrder.Payment.Amount, true, nil
		}
	}
}

func extractErrorMessage(m *pb.Message) error {
	errMsg := new(pb.Error)
	err := ptypes.UnmarshalAny(m.Payload, errMsg)
	if err == nil {
		// if the server sends back JSON don't format it
		var jsonObj map[string]interface{}
		if json.Unmarshal([]byte(errMsg.ErrorMessage), &jsonObj) == nil {
			return errors.New(errMsg.ErrorMessage)
		}

		return fmt.Errorf("Vendor rejected order, reason: %s", errMsg.ErrorMessage)
	} else { // For backwards compatibility check for a string payload
		return errors.New(string(m.Payload.Value))
	}
}

func (n *OpenBazaarNode) createContractWithOrder(data *PurchaseData) (*pb.RicardianContract, error) {
	var (
		contract = new(pb.RicardianContract)
		order    = new(pb.Order)

		shipping = &pb.Order_Shipping{
			ShipTo:       data.ShipTo,
			Address:      data.Address,
			City:         data.City,
			State:        data.State,
			PostalCode:   data.PostalCode,
			Country:      pb.CountryCode(pb.CountryCode_value[data.CountryCode]),
			AddressNotes: data.AddressNotes,
		}
	)

	contract.BuyerOrder = order
	order.Version = 2

	if data.RefundAddress != nil {
		order.RefundAddress = *(data.RefundAddress)
	} else {
		order.RefundAddress = n.Wallet.NewAddress(wallet.INTERNAL).EncodeAddress()
	}
	order.Shipping = shipping

	id := new(pb.ID)
	profile, err := n.GetProfile()
	if err == nil {
		id.Handle = profile.Handle
	}

	id.PeerID = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return nil, err
	}
	keys := new(pb.ID_Pubkeys)
	keys.Identity = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return nil, err
	}
	keys.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = keys
	// Sign the PeerID with the Bitcoin key
	ecPrivKey, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return nil, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.PeerID))
	id.BitcoinSig = sig.Serialize()
	order.BuyerID = id

	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return nil, err
	}
	order.Timestamp = ts
	order.AlternateContactInfo = data.AlternateContactInfo

	var ratingKeys [][]byte
	for range data.Items {
		// FIXME: bug here. This should use a different key for each item. This code doesn't look like it will do that.
		// Also the fix for this will also need to be included in the rating signing code.
		ratingKey, err := n.Wallet.MasterPublicKey().Child(uint32(ts.Seconds))
		if err != nil {
			return nil, err
		}
		ecRatingKey, err := ratingKey.ECPubKey()
		if err != nil {
			return nil, err
		}
		ratingKeys = append(ratingKeys, ecRatingKey.SerializeCompressed())
	}
	order.RatingKeys = ratingKeys

	addedListings := make(map[string]*pb.Listing)
	for _, item := range data.Items {
		i := new(pb.Order_Item)

		/* It is possible that multiple items could refer to the same listing if the buyer is ordering
		   multiple items with different variants. If it is multiple items of the same variant they can just
		   use the quantity field. But different variants require two separate item entries. However,
		   in this case we do not need to add the listing to the contract twice. Just once is sufficient.
		   So let's check to see if that's the case here and handle it. */
		_, exists := addedListings[item.ListingHash]

		listing := new(pb.Listing)
		if !exists {
			// Let's fetch the listing, should be cached
			b, err := ipfs.Cat(n.Context, item.ListingHash, time.Minute)
			if err != nil {
				return nil, err
			}
			sl := new(pb.SignedListing)
			err = jsonpb.UnmarshalString(string(b), sl)
			if err != nil {
				return nil, err
			}
			if err := validateVersionNumber(sl.Listing); err != nil {
				return nil, err
			}
			if err := validateVendorID(sl.Listing); err != nil {
				return nil, err
			}
			if err := validateListing(sl.Listing, n.TestNetworkEnabled() || n.RegressionNetworkEnabled()); err != nil {
				return nil, fmt.Errorf("Listing failed to validate, reason: %q", err.Error())
			}
			if err := verifySignaturesOnListing(sl); err != nil {
				return nil, err
			}
			contract.VendorListings = append(contract.VendorListings, sl.Listing)
			s := new(pb.Signature)
			s.Section = pb.Signature_LISTING
			s.SignatureBytes = sl.Signature
			contract.Signatures = append(contract.Signatures, s)
			addedListings[item.ListingHash] = sl.Listing
			listing = sl.Listing
		} else {
			listing = addedListings[item.ListingHash]
		}

		if strings.ToLower(listing.Metadata.AcceptedCurrencies[0]) != strings.ToLower(n.Wallet.CurrencyCode()) {
			return nil, fmt.Errorf("Contract only accepts %s, our wallet uses %s", listing.Metadata.AcceptedCurrencies[0], n.Wallet.CurrencyCode())
		}

		ser, err := proto.Marshal(listing)
		if err != nil {
			return nil, err
		}
		listingID, err := EncodeCID(ser)
		if err != nil {
			return nil, err
		}
		i.ListingHash = listingID.String()

		// If purchasing a listing version >=3 then the Quantity64 field must be used
		if listing.Metadata.Version < 3 {
			i.Quantity = uint32(item.Quantity)
		} else {
			i.Quantity64 = uint64(item.Quantity)
		}

		if listing.Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY {
			i.Memo = item.Memo

			// Remove any duplicate coupons
			couponMap := make(map[string]bool)
			var coupons []string
			for _, c := range item.Coupons {
				if !couponMap[c] {
					couponMap[c] = true
					coupons = append(coupons, c)
				}
			}
			i.CouponCodes = coupons

			// Validate the selected options
			listingOptions := make(map[string]*pb.Listing_Item_Option)
			for _, opt := range listing.Item.Options {
				listingOptions[strings.ToLower(opt.Name)] = opt
			}
			for _, uopt := range item.Options {
				_, ok := listingOptions[strings.ToLower(uopt.Name)]
				if !ok {
					return nil, errors.New("Selected variant not in listing")
				}
				delete(listingOptions, strings.ToLower(uopt.Name))
			}
			if len(listingOptions) > 0 {
				return nil, errors.New("Not all options were selected")
			}

			for _, option := range item.Options {
				o := &pb.Order_Item_Option{
					Name:  option.Name,
					Value: option.Value,
				}
				i.Options = append(i.Options, o)
			}
		}

		// Add shipping to physical listings, and include it for digital and service
		// listings for legacy compatibility
		if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD ||
			listing.Metadata.ContractType == pb.Listing_Metadata_DIGITAL_GOOD ||
			listing.Metadata.ContractType == pb.Listing_Metadata_SERVICE {

			i.ShippingOption = &pb.Order_Item_ShippingOption{
				Name:    item.Shipping.Name,
				Service: item.Shipping.Service,
			}
		}

		if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
			i.PaymentAddress = item.PaymentAddress
			validateCryptocurrencyOrderItem(i)
		}

		order.Items = append(order.Items, i)
	}

	if containsPhysicalGood(addedListings) && !(n.TestNetworkEnabled() || n.RegressionNetworkEnabled()) {
		err := validatePhysicalPurchaseOrder(contract)
		if err != nil {
			return nil, err
		}
	}

	return contract, nil
}

func containsPhysicalGood(addedListings map[string]*pb.Listing) bool {
	for _, listing := range addedListings {
		if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			return true
		}
	}
	return false
}

func validatePhysicalPurchaseOrder(contract *pb.RicardianContract) error {
	if contract.BuyerOrder.Shipping == nil {
		return errors.New("Order is missing shipping object")
	}
	if contract.BuyerOrder.Shipping.Address == "" {
		return errors.New("Shipping address is empty")
	}
	if contract.BuyerOrder.Shipping.ShipTo == "" {
		return errors.New("Ship to name is empty")
	}

	return nil
}

func validateCryptocurrencyOrderItem(item *pb.Order_Item) error {
	if len(item.Options) > 0 {
		return ErrCryptocurrencyPurchaseIllegalField("item.options")
	}
	if len(item.CouponCodes) > 0 {
		return ErrCryptocurrencyPurchaseIllegalField("item.couponCodes")
	}
	if item.PaymentAddress == "" {
		return ErrCryptocurrencyPurchasePaymentAddressRequired
	}
	if len(item.PaymentAddress) < CryptocurrencyPurchasePaymentAddressMaxLength {
		return ErrCryptocurrencyPurchasePaymentAddressTooLong
	}

	return nil
}

func (n *OpenBazaarNode) EstimateOrderTotal(data *PurchaseData) (uint64, error) {
	contract, err := n.createContractWithOrder(data)
	if err != nil {
		return 0, err
	}
	return n.CalculateOrderTotal(contract)
}

func (n *OpenBazaarNode) CancelOfflineOrder(contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}
	// Sweep the temp address into our wallet
	var utxos []wallet.Utxo
	for _, r := range records {
		if !r.Spent && r.Value > 0 {
			u := wallet.Utxo{}
			scriptBytes, err := hex.DecodeString(r.ScriptPubKey)
			if err != nil {
				return err
			}
			u.ScriptPubkey = scriptBytes
			hash, err := chainhash.NewHashFromStr(r.Txid)
			if err != nil {
				return err
			}
			outpoint := wire.NewOutPoint(hash, r.Index)
			u.Op = *outpoint
			u.Value = r.Value
			utxos = append(utxos, u)
		}
	}

	if len(utxos) == 0 {
		return errors.New("Cannot cancel order because utxo has already been spent")
	}

	chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
	if err != nil {
		return err
	}
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	mPrivKey := n.Wallet.MasterPrivateKey()
	if err != nil {
		return err
	}
	mECKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		return err
	}
	hdKey := hd.NewExtendedKey(
		n.Wallet.Params().HDPrivateKeyID[:],
		mECKey.Serialize(),
		chaincode,
		parentFP,
		0,
		0,
		true)

	buyerKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
	refundAddress, err := n.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
	if err != nil {
		return err
	}
	_, err = n.Wallet.SweepAddress(utxos, &refundAddress, buyerKey, &redeemScript, wallet.NORMAL)
	if err != nil {
		return err
	}
	err = n.SendCancel(contract.VendorListings[0].VendorID.PeerID, orderId)
	if err != nil {
		return err
	}
	n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_CANCELED, true)
	return nil
}

func (n *OpenBazaarNode) CalcOrderId(order *pb.Order) (string, error) {
	ser, err := proto.Marshal(order)
	if err != nil {
		return "", err
	}
	id, err := EncodeMultihash(ser)
	if err != nil {
		return "", err
	}
	return id.B58String(), nil
}

func (n *OpenBazaarNode) CalculateOrderTotal(contract *pb.RicardianContract) (uint64, error) {
	if n.ExchangeRates != nil {
		n.ExchangeRates.GetLatestRate("") // Refresh the exchange rates
	}

	var total uint64
	physicalGoods := make(map[string]*pb.Listing)

	// Calculate the price of each item
	for _, item := range contract.BuyerOrder.Items {
		var (
			satoshis     uint64
			itemTotal    uint64
			itemQuantity uint64
		)

		l, err := ParseContractForListing(item.ListingHash, contract)
		if err != nil {
			return 0, fmt.Errorf("Listing not found in contract for item %s", item.ListingHash)
		}

		// Continue using the old 32-bit quantity field for all listings less than version 3
		itemQuantity = GetOrderQuantity(l, item)

		if l.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			physicalGoods[item.ListingHash] = l
		}

		if l.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
			satoshis, err = n.getMarketPriceInSatoshis(l.Metadata.CoinType, itemQuantity)
			itemQuantity = 1
		} else {
			satoshis, err = n.getPriceInSatoshi(l.Metadata.PricingCurrency, l.Item.Price)
		}
		if err != nil {
			return 0, err
		}
		itemTotal += satoshis
		selectedSku, err := GetSelectedSku(l, item.Options)
		if err != nil {
			return 0, err
		}
		skuExists := false
		for i, sku := range l.Item.Skus {
			if selectedSku == i {
				skuExists = true
				if sku.Surcharge != 0 {
					surcharge := uint64(sku.Surcharge)
					if sku.Surcharge < 0 {
						surcharge = uint64(-sku.Surcharge)
					}
					satoshis, err := n.getPriceInSatoshi(l.Metadata.PricingCurrency, surcharge)
					if err != nil {
						return 0, err
					}
					if sku.Surcharge < 0 {
						itemTotal -= satoshis
					} else {
						itemTotal += satoshis
					}
				}
				if !skuExists {
					return 0, errors.New("Selected variant not found in listing")
				}
				break
			}
		}
		// Subtract any coupons
		for _, couponCode := range item.CouponCodes {
			for _, vendorCoupon := range l.Coupons {
				id, err := EncodeMultihash([]byte(couponCode))
				if err != nil {
					return 0, err
				}
				if id.B58String() == vendorCoupon.GetHash() {
					if discount := vendorCoupon.GetPriceDiscount(); discount > 0 {
						satoshis, err := n.getPriceInSatoshi(l.Metadata.PricingCurrency, discount)
						if err != nil {
							return 0, err
						}
						itemTotal -= satoshis
					} else if discount := vendorCoupon.GetPercentDiscount(); discount > 0 {
						itemTotal -= uint64((float32(itemTotal) * (discount / 100)))
					}
				}
			}
		}
		// Apply tax
		for _, tax := range l.Taxes {
			for _, taxRegion := range tax.TaxRegions {
				if contract.BuyerOrder.Shipping.Country == taxRegion {
					itemTotal += uint64((float32(itemTotal) * (tax.Percentage / 100)))
					break
				}
			}
		}
		itemTotal *= uint64(itemQuantity)
		total += itemTotal
	}

	shippingTotal, err := n.calculateShippingTotalForListings(contract, physicalGoods)
	if err != nil {
		return 0, err
	}
	total += shippingTotal

	return total, nil
}

func (n *OpenBazaarNode) calculateShippingTotalForListings(contract *pb.RicardianContract, listings map[string]*pb.Listing) (uint64, error) {
	type itemShipping struct {
		primary               uint64
		secondary             uint64
		quantity              uint32
		shippingTaxPercentage float32
		version               uint32
	}
	var (
		is            []itemShipping
		shippingTotal uint64
	)

	// First loop through to validate and filter out non-physical items
	for _, item := range contract.BuyerOrder.Items {
		listing, ok := listings[item.ListingHash]
		if !ok {
			continue
		}

		// Check selected option exists
		shippingOptions := make(map[string]*pb.Listing_ShippingOption)
		for _, so := range listing.ShippingOptions {
			shippingOptions[strings.ToLower(so.Name)] = so
		}
		option, ok := shippingOptions[strings.ToLower(item.ShippingOption.Name)]
		if !ok {
			return 0, errors.New("Shipping option not found in listing")
		}

		if option.Type == pb.Listing_ShippingOption_LOCAL_PICKUP {
			continue
		}

		// Check that this option ships to us
		regions := make(map[pb.CountryCode]bool)
		for _, country := range option.Regions {
			regions[country] = true
		}
		_, shipsToMe := regions[contract.BuyerOrder.Shipping.Country]
		_, shipsToAll := regions[pb.CountryCode_ALL]
		if !shipsToMe && !shipsToAll {
			return 0, errors.New("Listing does ship to selected country")
		}

		// Check service exists
		services := make(map[string]*pb.Listing_ShippingOption_Service)
		for _, shippingService := range option.Services {
			services[strings.ToLower(shippingService.Name)] = shippingService
		}
		service, ok := services[strings.ToLower(item.ShippingOption.Service)]
		if !ok {
			return 0, errors.New("Shipping service not found in listing")
		}
		shippingSatoshi, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, service.Price)
		if err != nil {
			return 0, err
		}

		var secondarySatoshi uint64
		if service.AdditionalItemPrice > 0 {
			secondarySatoshi, err = n.getPriceInSatoshi(listing.Metadata.PricingCurrency, service.AdditionalItemPrice)
			if err != nil {
				return 0, err
			}
		}

		// Calculate tax percentage
		var shippingTaxPercentage float32
		for _, tax := range listing.Taxes {
			regions := make(map[pb.CountryCode]bool)
			for _, taxRegion := range tax.TaxRegions {
				regions[taxRegion] = true
			}
			_, ok := regions[contract.BuyerOrder.Shipping.Country]
			if ok && tax.TaxShipping {
				shippingTaxPercentage = tax.Percentage / 100
			}
		}

		is = append(is, itemShipping{
			primary:               shippingSatoshi,
			secondary:             secondarySatoshi,
			quantity:              item.Quantity,
			shippingTaxPercentage: shippingTaxPercentage,
			version:               listing.Metadata.Version,
		})
	}

	if len(is) == 0 {
		return 0, nil
	}

	if len(is) == 1 {
		shippingTotal = (is[0].primary * uint64(((1+is[0].shippingTaxPercentage)*100)+.5) / 100)
		if is[0].quantity > 1 {
			if is[0].version == 1 {
				shippingTotal += (is[0].primary * uint64(((1+is[0].shippingTaxPercentage)*100)+.5) / 100) * uint64((is[0].quantity - 1))
			} else if is[0].version >= 2 {
				shippingTotal += (is[0].secondary * uint64(((1+is[0].shippingTaxPercentage)*100)+.5) / 100) * uint64((is[0].quantity - 1))
			} else {
				return 0, errors.New("Unknown listing version")
			}
		}
		return shippingTotal, nil
	}

	var highest uint64
	var i int
	for x, s := range is {
		if s.primary > highest {
			highest = s.primary
			i = x
		}
		shippingTotal += (s.secondary * uint64(((1+s.shippingTaxPercentage)*100)+.5) / 100) * uint64(s.quantity)
	}
	shippingTotal -= (is[i].primary * uint64(((1+is[i].shippingTaxPercentage)*100)+.5) / 100)
	shippingTotal += (is[i].secondary * uint64(((1+is[i].shippingTaxPercentage)*100)+.5) / 100)

	return shippingTotal, nil
}

func (n *OpenBazaarNode) getPriceInSatoshi(currencyCode string, amount uint64) (uint64, error) {
	if strings.ToLower(currencyCode) == strings.ToLower(n.Wallet.CurrencyCode()) || "t"+strings.ToLower(currencyCode) == strings.ToLower(n.Wallet.CurrencyCode()) {
		return amount, nil
	}

	if n.ExchangeRates == nil {
		return 0, ErrPriceCalculationRequiresExchangeRates
	}
	exchangeRate, err := n.ExchangeRates.GetExchangeRate(currencyCode)
	if err != nil {
		return 0, err
	}

	formatedAmount := float64(amount) / 100
	btc := formatedAmount / exchangeRate
	satoshis := btc * float64(n.ExchangeRates.UnitsPerCoin())
	return uint64(satoshis), nil
}

func (n *OpenBazaarNode) getMarketPriceInSatoshis(currencyCode string, amount uint64) (uint64, error) {
	if n.ExchangeRates == nil {
		return 0, ErrPriceCalculationRequiresExchangeRates
	}

	rate, err := n.ExchangeRates.GetExchangeRate(currencyCode)
	if err != nil {
		return 0, err
	}

	return uint64(float64(amount) / rate), nil
}

func verifySignaturesOnOrder(contract *pb.RicardianContract) error {
	if err := verifyMessageSignature(
		contract.BuyerOrder,
		contract.BuyerOrder.BuyerID.Pubkeys.Identity,
		contract.Signatures,
		pb.Signature_ORDER,
		contract.BuyerOrder.BuyerID.PeerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the order")
		case invalidSigError:
			return errors.New("Buyer's identity signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in order does not match reported buyer ID")
		default:
			return err
		}
	}

	if err := verifyBitcoinSignature(
		contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
		contract.BuyerOrder.BuyerID.BitcoinSig,
		contract.BuyerOrder.BuyerID.PeerID,
	); err != nil {
		switch err.(type) {
		case invalidSigError:
			return errors.New("Buyer's bitcoin signature on GUID failed to verify")
		default:
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) ValidateOrder(contract *pb.RicardianContract, checkInventory bool) error {
	listingMap := make(map[string]*pb.Listing)

	// Check order contains all required fields
	if contract.BuyerOrder == nil {
		return errors.New("Contract doesn't contain an order")
	}
	if contract.BuyerOrder.Payment == nil {
		return errors.New("Order doesn't contain a payment")
	}
	if contract.BuyerOrder.BuyerID == nil {
		return errors.New("Order doesn't contain a buyer ID")
	}
	if len(contract.BuyerOrder.Items) == 0 {
		return errors.New("Order hasn't selected any items")
	}
	if len(contract.BuyerOrder.RatingKeys) != len(contract.BuyerOrder.Items) {
		return errors.New("Number of rating keys do not match number of items")
	}
	for _, ratingKey := range contract.BuyerOrder.RatingKeys {
		if len(ratingKey) != 33 {
			return errors.New("Invalid rating key in order")
		}
	}
	if contract.BuyerOrder.Timestamp == nil {
		return errors.New("Order is missing a timestamp")
	}
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		_, err := mh.FromB58String(contract.BuyerOrder.Payment.Moderator)
		if err != nil {
			return errors.New("Invalid moderator")
		}
		var availableMods []string
		for _, listing := range contract.VendorListings {
			availableMods = append(availableMods, listing.Moderators...)
		}
		validMod := false
		for _, mod := range availableMods {
			if mod == contract.BuyerOrder.Payment.Moderator {
				validMod = true
				break
			}
		}
		if !validMod {
			return errors.New("Invalid moderator")
		}
	}

	// Validate that the hash of the items in the contract match claimed hash in the order
	// itemHashes should avoid duplicates
	var itemHashes []string
collectListings:
	for _, item := range contract.BuyerOrder.Items {
		for _, hash := range itemHashes {
			if hash == item.ListingHash {
				continue collectListings
			}
		}
		itemHashes = append(itemHashes, item.ListingHash)
	}
	// TODO: use function for this
	for _, listing := range contract.VendorListings {
		ser, err := proto.Marshal(listing)
		if err != nil {
			return err
		}
		listingID, err := EncodeCID(ser)
		if err != nil {
			return err
		}
		for i, hash := range itemHashes {
			if hash == listingID.String() {
				itemHashes = append(itemHashes[:i], itemHashes[i+1:]...)
				listingMap[hash] = listing
			}
		}
	}
	if len(itemHashes) > 0 {
		return errors.New("Item hashes in the order do not match the included listings")
	}

	// Validate no duplicate coupons
	for _, item := range contract.BuyerOrder.Items {
		couponMap := make(map[string]bool)
		for _, c := range item.CouponCodes {
			if couponMap[c] {
				return errors.New("Duplicate coupon code in order")
			}
			couponMap[c] = true
		}
	}

	// Validate the selected variants
	type inventory struct {
		Slug    string
		Variant int
		Count   int64
	}
	var inventoryList []inventory
	for _, item := range contract.BuyerOrder.Items {
		var userOptions []*pb.Order_Item_Option
		var listingOptions []string
		for _, opt := range listingMap[item.ListingHash].Item.Options {
			listingOptions = append(listingOptions, opt.Name)
		}
		for _, uopt := range item.Options {
			userOptions = append(userOptions, uopt)
		}
		inv := inventory{Slug: listingMap[item.ListingHash].Slug}
		selectedVariant, err := GetSelectedSku(listingMap[item.ListingHash], item.Options)
		if err != nil {
			return err
		}
		inv.Variant = selectedVariant
		for _, o := range listingMap[item.ListingHash].Item.Options {
			for _, checkOpt := range userOptions {
				if strings.ToLower(o.Name) == strings.ToLower(checkOpt.Name) {
					var validVariant bool = false
					for _, v := range o.Variants {
						if strings.ToLower(v.Name) == strings.ToLower(checkOpt.Value) {
							validVariant = true
						}
					}
					if validVariant == false {
						return errors.New("Selected variant not in listing")
					}
				}
			check:
				for i, lopt := range listingOptions {
					if strings.ToLower(checkOpt.Name) == strings.ToLower(lopt) {
						listingOptions = append(listingOptions[:i], listingOptions[i+1:]...)
						continue check
					}
				}
			}
		}
		if len(listingOptions) > 0 {
			return errors.New("Not all options were selected")
		}
		// Create inventory paths to check later
		inv.Count = int64(GetOrderQuantity(listingMap[item.ListingHash], item))
		inventoryList = append(inventoryList, inv)
	}

	// Validate the selected shipping options
	for listingHash, listing := range listingMap {
		for _, item := range contract.BuyerOrder.Items {
			if item.ListingHash == listingHash {
				if listing.Metadata.ContractType != pb.Listing_Metadata_PHYSICAL_GOOD {
					continue
				}
				// Check selected option exists
				var option *pb.Listing_ShippingOption
				for _, shippingOption := range listing.ShippingOptions {
					if shippingOption.Name == item.ShippingOption.Name {
						option = shippingOption
						break
					}
				}
				if option == nil {
					return errors.New("Shipping option not found in listing")
				}

				// Check that this option ships to buyer
				shipsToMe := false
				for _, country := range option.Regions {
					if country == contract.BuyerOrder.Shipping.Country || country == pb.CountryCode_ALL {
						shipsToMe = true
						break
					}
				}
				if !shipsToMe {
					return errors.New("Listing does ship to selected country")
				}

				// Check service exists
				if option.Type != pb.Listing_ShippingOption_LOCAL_PICKUP {
					var service *pb.Listing_ShippingOption_Service
					for _, shippingService := range option.Services {
						if strings.ToLower(shippingService.Name) == strings.ToLower(item.ShippingOption.Service) {
							service = shippingService
						}
					}
					if service == nil {
						return errors.New("Shipping service not found in listing")
					}
				}
				break
			}
		}
	}

	// Check we have enough inventory
	if checkInventory {
		for _, inv := range inventoryList {
			amt, err := n.Datastore.Inventory().GetSpecific(inv.Slug, inv.Variant)
			if err != nil {
				return errors.New("Vendor has no inventory for the selected variant.")
			}
			if amt >= 0 && amt < inv.Count {
				return NewErrOutOfInventory(amt)
			}
		}
	}

	// Validate shipping
	containsPhysicalGood := false
	for _, listing := range listingMap {
		if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			containsPhysicalGood = true
			break
		}
	}
	if containsPhysicalGood {
		if contract.BuyerOrder.Shipping == nil {
			return errors.New("Order is missing shipping object")
		}
		if contract.BuyerOrder.Shipping.Address == "" {
			return errors.New("Shipping address is empty")
		}
		if contract.BuyerOrder.Shipping.ShipTo == "" {
			return errors.New("Ship to name is empty")
		}
	}

	// Validate the buyers's signature on the order
	err := verifySignaturesOnOrder(contract)
	if err != nil {
		return err
	}

	// Validate the each item in the order is for sale
	if !n.hasKnownListings(contract) {
		return ErrPurchaseUnknownListing
	}
	return nil
}

func (n *OpenBazaarNode) hasKnownListings(contract *pb.RicardianContract) bool {
	for _, listing := range contract.VendorListings {
		if !n.IsItemForSale(listing) {
			return false
		}
	}
	return true
}

func (n *OpenBazaarNode) ValidateDirectPaymentAddress(order *pb.Order) error {
	chaincode, err := hex.DecodeString(order.Payment.Chaincode)
	if err != nil {
		return err
	}
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	mECKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return err
	}
	hdKey := hd.NewExtendedKey(
		n.Wallet.Params().HDPublicKeyID[:],
		mECKey.SerializeCompressed(),
		chaincode,
		parentFP,
		0,
		0,
		false)

	vendorKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	hdKey = hd.NewExtendedKey(
		n.Wallet.Params().HDPublicKeyID[:],
		order.BuyerID.Pubkeys.Bitcoin,
		chaincode,
		parentFP,
		0,
		0,
		false)

	buyerKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey}, 1, time.Duration(0), nil)
	if order.Payment.Address != addr.EncodeAddress() {
		return errors.New("Invalid payment address")
	}
	if order.Payment.RedeemScript != hex.EncodeToString(redeemScript) {
		return errors.New("Invalid redeem script")
	}
	return nil
}

func (n *OpenBazaarNode) ValidateModeratedPaymentAddress(order *pb.Order, timeout time.Duration) error {
	ipnsPath := ipfspath.FromString(order.Payment.Moderator + "/profile.json")
	profileBytes, err := n.IPNSResolveThenCat(ipnsPath, time.Minute)
	if err != nil {
		return err
	}
	profile := new(pb.Profile)
	err = jsonpb.UnmarshalString(string(profileBytes), profile)
	if err != nil {
		return err
	}
	moderatorBytes, err := hex.DecodeString(profile.BitcoinPubkey)
	if err != nil {
		return err
	}

	chaincode, err := hex.DecodeString(order.Payment.Chaincode)
	if err != nil {
		return err
	}
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	mECKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return err
	}
	hdKey := hd.NewExtendedKey(
		n.Wallet.Params().HDPublicKeyID[:],
		mECKey.SerializeCompressed(),
		chaincode,
		parentFP,
		0,
		0,
		false)

	vendorKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	hdKey = hd.NewExtendedKey(
		n.Wallet.Params().HDPublicKeyID[:],
		order.BuyerID.Pubkeys.Bitcoin,
		chaincode,
		parentFP,
		0,
		0,
		false)

	buyerKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	hdKey = hd.NewExtendedKey(
		n.Wallet.Params().HDPublicKeyID[:],
		moderatorBytes,
		chaincode,
		parentFP,
		0,
		0,
		false)

	ModeratorKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	ModPub, err := ModeratorKey.ECPubKey()
	if err != nil {
		return err
	}
	if !bytes.Equal(order.Payment.ModeratorKey, ModPub.SerializeCompressed()) {
		return errors.New("Invalid moderator key")
	}
	addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *ModeratorKey}, 2, timeout, vendorKey)
	if order.Payment.Address != addr.EncodeAddress() {
		return errors.New("Invalid payment address")
	}
	if order.Payment.RedeemScript != hex.EncodeToString(redeemScript) {
		return errors.New("Invalid redeem script")
	}
	return nil
}

func (n *OpenBazaarNode) SignOrder(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrder, err := proto.Marshal(contract.BuyerOrder)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_ORDER
	if err != nil {
		return contract, err
	}
	idSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrder)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = idSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func validateVendorID(listing *pb.Listing) error {

	if listing == nil {
		return errors.New("Listing is nil")
	}
	if listing.VendorID == nil {
		return errors.New("VendorID is nil")
	}
	if listing.VendorID.Pubkeys == nil {
		return errors.New("Vendor pubkeys is nil")
	}
	vendorPubKey, err := crypto.UnmarshalPublicKey(listing.VendorID.Pubkeys.Identity)
	if err != nil {
		return err
	}
	vendorId, err := peer.IDB58Decode(listing.VendorID.PeerID)
	if err != nil {
		return err
	}
	if !vendorId.MatchesPublicKey(vendorPubKey) {
		return errors.New("Invalid vendor ID")
	}
	return nil
}

func validateVersionNumber(listing *pb.Listing) error {
	if listing == nil {
		return errors.New("Listing is nil")
	}
	if listing.Metadata == nil {
		return errors.New("Listing does not contain metadata")
	}
	if listing.Metadata.Version > ListingVersion {
		return errors.New("Unknown listing version. You must upgrade to purchase this listing.")
	}
	return nil
}

func (n *OpenBazaarNode) ValidatePaymentAmount(requestedAmount, paymentAmount uint64) bool {
	settings, _ := n.Datastore.Settings().Get()
	bufferPercent := float32(0)
	if settings.MisPaymentBuffer != nil {
		bufferPercent = *settings.MisPaymentBuffer
	}
	buffer := float32(requestedAmount) * (bufferPercent / 100)
	if float32(paymentAmount)+buffer < float32(requestedAmount) {
		return false
	}
	return true
}

func ParseContractForListing(hash string, contract *pb.RicardianContract) (*pb.Listing, error) {
	for _, listing := range contract.VendorListings {
		ser, err := proto.Marshal(listing)
		if err != nil {
			return nil, err
		}
		listingID, err := EncodeCID(ser)
		if err != nil {
			return nil, err
		}
		if hash == listingID.String() {
			return listing, nil
		}
	}
	return nil, errors.New("Not found")
}

func GetSelectedSku(listing *pb.Listing, itemOptions []*pb.Order_Item_Option) (int, error) {
	if len(itemOptions) == 0 && (len(listing.Item.Skus) == 1 || len(listing.Item.Skus) == 0) {
		// Default sku
		return 0, nil
	}
	var selected []int
	for _, s := range listing.Item.Options {
	optionsLoop:
		for _, o := range itemOptions {
			if strings.ToLower(o.Name) == strings.ToLower(s.Name) {
				for i, va := range s.Variants {
					if strings.ToLower(va.Name) == strings.ToLower(o.Value) {
						selected = append(selected, i)
						break optionsLoop
					}
				}
			}
		}
	}
	for i, sku := range listing.Item.Skus {
		if SameSku(selected, sku) {
			return i, nil
		}
	}
	return 0, errors.New("No skus selected")
}

func SameSku(selectedVariants []int, sku *pb.Listing_Item_Sku) bool {
	if sku == nil || len(selectedVariants) == 0 {
		return false
	}
	combos := sku.VariantCombo
	if len(selectedVariants) != len(combos) {
		return false
	}

	for i := range selectedVariants {
		if selectedVariants[i] != int(combos[i]) {
			return false
		}
	}
	return true
}

func GetOrderQuantity(l *pb.Listing, item *pb.Order_Item) uint64 {
	if l.Metadata.Version < 3 {
		return uint64(item.Quantity)
	}
	return item.Quantity64
}
