package core

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	ipfspath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/wallet-interface"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

const (
	// We use this to check to see if the approximate fee to release funds from escrow is greater than 1/4th of the amount
	// being released. If so, we prevent the purchase from being made as it severely cuts into the vendor's profits.
	// TODO: this probably should not be hardcoded but making it adaptive requires all wallet implementations to provide this data.
	// TODO: for now, this is probably OK as it's just an approximation.

	// EscrowReleaseSize - size in bytes for escrow op
	EscrowReleaseSize = 337
	// CryptocurrencyPurchasePaymentAddressMaxLength - max permissible length for an address
	CryptocurrencyPurchasePaymentAddressMaxLength = 512
)

// GetOrder - provide API response order object by orderID
func (n *OpenBazaarNode) GetOrder(orderID string) (*pb.OrderRespApi, error) {
	var (
		err      error
		isSale   bool
		contract *pb.RicardianContract
		state    pb.OrderState
		funded   bool
		records  []*wallet.TransactionRecord
		read     bool
		//paymentCoin *repo.CurrencyCode
	)
	contract, state, funded, records, read, _, err = n.Datastore.Purchases().GetByOrderId(orderID)
	if err != nil {
		contract, state, funded, records, read, _, err = n.Datastore.Sales().GetByOrderId(orderID)
		if err != nil {
			return nil, errors.New("order not found")
		}
		isSale = true
	}
	resp := new(pb.OrderRespApi)
	resp.Contract = contract
	resp.Funded = funded
	resp.Read = read
	resp.State = state

	// TODO: Remove once broken contracts are migrated
	lookupCoin := contract.BuyerOrder.Payment.AmountCurrency.Code
	_, err = n.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, orderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
	}

	paymentTxs, refundTx, err := n.BuildTransactionRecords(contract, records, state)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}
	resp.PaymentAddressTransactions = paymentTxs
	resp.RefundAddressTransaction = refundTx

	unread, err := n.Datastore.Chat().GetUnreadCount(orderID)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}
	resp.UnreadChatMessages = uint64(unread)

	if isSale {
		err = n.Datastore.Sales().MarkAsRead(orderID)
		if err != nil {
			log.Error(err)
		}
	} else {
		err = n.Datastore.Purchases().MarkAsRead(orderID)
		if err != nil {
			log.Error(err)
		}
	}

	return resp, nil
}

// Purchase - add ricardian contract
func (n *OpenBazaarNode) Purchase(data *repo.PurchaseData) (orderID string, paymentAddress string, paymentAmount *repo.CurrencyValue, vendorOnline bool, err error) {
	retCurrency := &repo.CurrencyValue{}
	defn, err := n.LookupCurrency(data.PaymentCoin)
	if err != nil {
		return "", "", retCurrency, false, err
	}
	retCurrency.Currency = defn
	contract, err := n.createContractWithOrder(data)
	if err != nil {
		return "", "", retCurrency, false, err
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(data.PaymentCoin)
	if err != nil {
		return "", "", retCurrency, false, err
	}
	// Add payment data and send to vendor
	if data.Moderator != "" { // Moderated payment
		contract, err := prepareModeratedOrderContract(data, n, contract, wal)
		if err != nil {
			return "", "", retCurrency, false, err
		}

		contract, err = n.SignOrder(contract)
		if err != nil {
			return "", "", retCurrency, false, err
		}

		// Send to order vendor
		merchantResponse, err := n.SendOrder(contract.VendorListings[0].VendorID.PeerID, contract)
		if err != nil {
			id, addr, amt, err := processOfflineModeratedOrder(n, contract)
			retCurrency.Amount = &amt
			return id, addr, retCurrency, false, err
		}
		id, addr, amt, f, err := processOnlineModeratedOrder(merchantResponse, n, contract)
		retCurrency.Amount = &amt
		return id, addr, retCurrency, f, err

	}

	// Direct payment
	payment := new(pb.Order_Payment)
	payment.Method = pb.Order_Payment_ADDRESS_REQUEST

	contract.BuyerOrder.Payment = payment
	payment.AmountCurrency = &pb.CurrencyDefinition{
		Code:         defn.Code.String(),
		Divisibility: uint32(defn.Divisibility),
	}

	// Calculate payment amount
	total, err := n.CalculateOrderTotal(contract)
	if err != nil {
		return "", "", retCurrency, false, err
	}

	if wal.IsDust(*total) {
		return "", "", retCurrency, false, ErrSpendAmountIsDust
	}

	payment.BigAmount = total.String()

	contract, err = n.SignOrder(contract)
	if err != nil {
		return "", "", retCurrency, false, err
	}

	// Send to order vendor and request a payment address
	merchantResponse, err := n.SendOrder(contract.VendorListings[0].VendorID.PeerID, contract)
	if err != nil {
		id, addr, amount, err := processOfflineDirectOrder(n, wal, contract, payment)
		retCurrency.Amount = &amount
		return id, addr, retCurrency, false, err
	}
	id, addr, amt, f, err := processOnlineDirectOrder(merchantResponse, n, wal, contract)
	retCurrency.Amount = &amt
	return id, addr, retCurrency, f, err
}

func prepareModeratedOrderContract(data *repo.PurchaseData, n *OpenBazaarNode, contract *pb.RicardianContract, wal wallet.Wallet) (*pb.RicardianContract, error) {
	if data.Moderator == n.IpfsNode.Identity.Pretty() {
		return nil, errors.New("cannot select self as moderator")
	}
	if data.Moderator == contract.VendorListings[0].VendorID.PeerID {
		return nil, errors.New("cannot select vendor as moderator")
	}
	payment := new(pb.Order_Payment)
	payment.Method = pb.Order_Payment_MODERATED
	payment.Moderator = data.Moderator
	defn, err := n.LookupCurrency(data.PaymentCoin)
	if err != nil {
		return nil, errors.New("invalid payment coin")
	}
	payment.AmountCurrency = &pb.CurrencyDefinition{
		Code:         defn.Code.String(),
		Divisibility: uint32(defn.Divisibility),
	}

	profile, err := n.FetchProfile(data.Moderator, true)
	if err != nil {
		return nil, errors.New("moderator could not be found")
	}
	moderatorKeyBytes, err := hex.DecodeString(profile.BitcoinPubkey)
	if err != nil {
		return nil, err
	}
	if !profile.Moderator || profile.ModeratorInfo == nil || len(profile.ModeratorInfo.AcceptedCurrencies) == 0 {
		return nil, errors.New("moderator is not capable of moderating this transaction")
	}

	if !n.currencyInAcceptedCurrenciesList(data.PaymentCoin, profile.ModeratorInfo.AcceptedCurrencies) {
		return nil, errors.New("moderator does not accept our currency")
	}
	contract.BuyerOrder.Payment = payment
	payment.AmountCurrency = &pb.CurrencyDefinition{
		Code:         defn.Code.String(),
		Divisibility: uint32(defn.Divisibility),
	}
	total, err := n.CalculateOrderTotal(contract)
	if err != nil {
		return nil, err
	}
	payment.BigAmount = total.String()
	contract.BuyerOrder.Payment = payment

	fpb := wal.GetFeePerByte(wallet.NORMAL)
	f := new(big.Int).Mul(&fpb, big.NewInt(int64(EscrowReleaseSize)))
	t := new(big.Int).Div(total, big.NewInt(4))

	if f.Cmp(t) > 0 {
		return nil, errors.New("transaction fee too high for moderated payment")
	}

	/* Generate a payment address using the first child key derived from the buyers's,
	   vendors's and moderator's masterPubKey and a random chaincode. */
	chaincode := make([]byte, 32)
	_, err = rand.Read(chaincode)
	if err != nil {
		return nil, err
	}
	vendorKey, err := wal.ChildKey(contract.VendorListings[0].VendorID.Pubkeys.Bitcoin, chaincode, false)
	if err != nil {
		return nil, err
	}
	buyerKey, err := wal.ChildKey(contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin, chaincode, false)
	if err != nil {
		return nil, err
	}
	moderatorKey, err := wal.ChildKey(moderatorKeyBytes, chaincode, false)
	if err != nil {
		return nil, err
	}
	modPub, err := moderatorKey.ECPubKey()
	if err != nil {
		return nil, err
	}
	payment.ModeratorKey = modPub.SerializeCompressed()

	timeout, err := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
	if err != nil {
		return nil, err
	}
	addr, redeemScript, err := wal.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2, timeout, vendorKey)
	if err != nil {
		return nil, err
	}
	payment.Address = addr.EncodeAddress()
	payment.RedeemScript = hex.EncodeToString(redeemScript)
	payment.Chaincode = hex.EncodeToString(chaincode)
	fee := wal.GetFeePerByte(wallet.NORMAL)
	contract.BuyerOrder.BigRefundFee = fee.String()

	err = wal.AddWatchedAddresses(addr)
	if err != nil {
		return nil, err
	}
	return contract, nil
}

func processOnlineDirectOrder(resp *pb.Message, n *OpenBazaarNode, wal wallet.Wallet, contract *pb.RicardianContract) (string, string, big.Int, bool, error) {
	// Vendor responded
	if resp.MessageType == pb.Message_ERROR {
		return "", "", *big.NewInt(0), false, extractErrorMessage(resp)
	}
	if resp.MessageType != pb.Message_ORDER_CONFIRMATION {
		return "", "", *big.NewInt(0), false, errors.New("vendor responded to the order with an incorrect message type")
	}
	if resp.Payload == nil {
		return "", "", *big.NewInt(0), false, errors.New("vendor responded with nil payload")
	}
	rc := new(pb.RicardianContract)
	err := proto.Unmarshal(resp.Payload.Value, rc)
	if err != nil {
		return "", "", *big.NewInt(0), false, errors.New("error parsing the vendor's response")
	}
	contract.VendorOrderConfirmation = rc.VendorOrderConfirmation
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_CONFIRMATION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}
	err = n.ValidateOrderConfirmation(contract, true)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	addr, err := wal.DecodeAddress(contract.VendorOrderConfirmation.PaymentAddress)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	err = wal.AddWatchedAddresses(addr)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	err = n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	total, ok := new(big.Int).SetString(contract.BuyerOrder.Payment.BigAmount, 10)
	if !ok {
		return "", "", *big.NewInt(0), false, errors.New("invalid payment amount")
	}
	return orderID, contract.VendorOrderConfirmation.PaymentAddress, *total, true, nil
}

func processOfflineDirectOrder(n *OpenBazaarNode, wal wallet.Wallet, contract *pb.RicardianContract, payment *pb.Order_Payment) (string, string, big.Int, error) {
	// Vendor offline
	// Change payment code to direct

	total, ok := new(big.Int).SetString(contract.BuyerOrder.Payment.BigAmount, 10)
	if !ok {
		return "", "", *big.NewInt(0), errors.New("invalid payment amount")
	}
	fpb := wal.GetFeePerByte(wallet.NORMAL)
	f := new(big.Int).Mul(&fpb, big.NewInt(int64(EscrowReleaseSize)))
	t := new(big.Int).Div(total, big.NewInt(4))

	if f.Cmp(t) > 0 {
		return "", "", *big.NewInt(0), errors.New("transaction fee too high for offline 2of2 multisig payment")
	}
	payment.Method = pb.Order_Payment_DIRECT

	/* Generate a payment address using the first child key derived from the buyer's
	   and vendors's masterPubKeys and a random chaincode. */
	chaincode := make([]byte, 32)
	_, err := rand.Read(chaincode)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	vendorKey, err := wal.ChildKey(contract.VendorListings[0].VendorID.Pubkeys.Bitcoin, chaincode, false)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	buyerKey, err := wal.ChildKey(contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin, chaincode, false)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	addr, redeemScript, err := wal.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey}, 1, time.Duration(0), nil)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	payment.Address = addr.EncodeAddress()
	payment.RedeemScript = hex.EncodeToString(redeemScript)
	payment.Chaincode = hex.EncodeToString(chaincode)

	err = wal.AddWatchedAddresses(addr)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}

	// Remove signature and resign
	contract.Signatures = []*pb.Signature{contract.Signatures[0]}
	contract, err = n.SignOrder(contract)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}

	// Send using offline messaging
	log.Warningf("Vendor %s is offline, sending offline order message", contract.VendorListings[0].VendorID.PeerID)
	peerID, err := peer.IDB58Decode(contract.VendorListings[0].VendorID.PeerID)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	any, err := ptypes.MarshalAny(contract)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER,
		Payload:     any,
	}
	k, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	err = n.SendOfflineMessage(peerID, &k, &m)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	err = n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	return orderID, contract.BuyerOrder.Payment.Address, *total, err
}

func processOnlineModeratedOrder(resp *pb.Message, n *OpenBazaarNode, contract *pb.RicardianContract) (string, string, big.Int, bool, error) {
	// Vendor responded
	if resp.MessageType == pb.Message_ERROR {
		return "", "", *big.NewInt(0), false, extractErrorMessage(resp)
	}
	if resp.MessageType != pb.Message_ORDER_CONFIRMATION {
		return "", "", *big.NewInt(0), false, errors.New("vendor responded to the order with an incorrect message type")
	}
	rc := new(pb.RicardianContract)
	err := proto.Unmarshal(resp.Payload.Value, rc)
	if err != nil {
		return "", "", *big.NewInt(0), false, errors.New("error parsing the vendor's response")
	}
	contract.VendorOrderConfirmation = rc.VendorOrderConfirmation
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_CONFIRMATION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}
	err = n.ValidateOrderConfirmation(contract, true)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	if contract.VendorOrderConfirmation.PaymentAddress != contract.BuyerOrder.Payment.Address {
		return "", "", *big.NewInt(0), false, errors.New("vendor responded with incorrect multisig address")
	}
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	err = n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
	if err != nil {
		return "", "", *big.NewInt(0), false, err
	}
	total, ok := new(big.Int).SetString(contract.BuyerOrder.Payment.BigAmount, 10)
	if !ok {
		return "", "", *big.NewInt(0), false, errors.New("invalid payment amount")
	}
	return orderID, contract.VendorOrderConfirmation.PaymentAddress, *total, true, nil
}

func processOfflineModeratedOrder(n *OpenBazaarNode, contract *pb.RicardianContract) (string, string, big.Int, error) {
	// Vendor offline
	// Send using offline messaging
	log.Warningf("Vendor %s is offline, sending offline order message", contract.VendorListings[0].VendorID.PeerID)
	peerID, err := peer.IDB58Decode(contract.VendorListings[0].VendorID.PeerID)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	any, err := ptypes.MarshalAny(contract)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER,
		Payload:     any,
	}
	k, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	err = n.SendOfflineMessage(peerID, &k, &m)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return "", "", *big.NewInt(0), err
	}
	err = n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
	if err != nil {
		log.Error(err)
	}
	total, ok := new(big.Int).SetString(contract.BuyerOrder.Payment.BigAmount, 10)
	if !ok {
		return "", "", *big.NewInt(0), errors.New("invalid payment amount")
	}
	return orderID, contract.BuyerOrder.Payment.Address, *total, err
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

		return fmt.Errorf("vendor rejected order, reason: %s", errMsg.ErrorMessage)
	}
	// For backwards compatibility check for a string payload
	return errors.New(string(m.Payload.Value))
}

func (n *OpenBazaarNode) createContractWithOrder(data *repo.PurchaseData) (*pb.RicardianContract, error) {
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
	wal, err := n.Multiwallet.WalletForCurrencyCode(data.PaymentCoin)
	if err != nil {
		return nil, err
	}

	contract.BuyerOrder = order
	order.Version = 2
	order.Shipping = shipping
	order.AlternateContactInfo = data.AlternateContactInfo

	if data.RefundAddress != nil {
		order.RefundAddress = *(data.RefundAddress)
	} else {
		order.RefundAddress = wal.NewAddress(wallet.INTERNAL).EncodeAddress()
	}

	contractIdentity, err := getContractIdentity(n)
	if err != nil {
		return nil, err
	}
	order.BuyerID = &contractIdentity

	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return nil, err
	}
	order.Timestamp = ts

	ratingKeys, err := getRatingKeysForOrder(data, n, ts)
	if err != nil {
		return nil, err
	}
	order.RatingKeys = ratingKeys

	addedListings := make(map[string]*repo.Listing)
	for _, item := range data.Items {
		i := new(pb.Order_Item)

		/* It is possible that multiple items could refer to the same listing if the buyer is ordering
		   multiple items with different variants. If it is multiple items of the same variant they can just
		   use the quantity field. But different variants require two separate item entries. However,
		   in this case we do not need to add the listing to the contract twice. Just once is sufficient.
		   So let's check to see if that's the case here and handle it. */
		_, exists := addedListings[item.ListingHash]

		var listing *repo.Listing
		if !exists {
			sl, err := getSignedListing(n, contract, item)
			if err != nil {
				return nil, err
			}
			addedListings[item.ListingHash] = sl
			listing = sl
		} else {
			listing = addedListings[item.ListingHash]
		}

		acceptedCurrencies, err := listing.GetAcceptedCurrencies()
		if err != nil || !n.currencyInAcceptedCurrenciesList(data.PaymentCoin, acceptedCurrencies) {
			return nil, errors.New("listing does not accept the selected currency")
		}

		ser, err := proto.Marshal(listing.ProtoListing)
		if err != nil {
			return nil, err
		}
		listingID, err := ipfs.EncodeCID(ser)
		if err != nil {
			return nil, err
		}
		i.ListingHash = listingID.String()

		// set quantity according to schema version
		// TODO: extract to repo package model
		switch listing.ProtoListing.Metadata.Version {
		case 5:
			i.BigQuantity = item.Quantity
		case 4, 3:
			q, ok := new(big.Int).SetString(item.Quantity, 10)
			if !ok {
				return nil, errors.New("invalid quantity format")
			}
			i.Quantity64 = q.Uint64()
		default:
			q, ok := new(big.Int).SetString(item.Quantity, 10)
			if !ok {
				return nil, errors.New("invalid quantity format")
			}
			i.Quantity = uint32(q.Uint64())
		}

		i.Memo = item.Memo

		contractType, err := listing.GetContractType()
		if err != nil {
			return nil, err
		}

		if contractType != pb.Listing_Metadata_CRYPTOCURRENCY.String() {
			// Remove any duplicate coupons
			i.CouponCodes = dedupeCoupons(item.Coupons)

			// Validate the selected options
			validatedOptions, err := repo.ValidateListingOptions(listing.ProtoListing.Item.Options, item.Options)
			if err != nil {
				return nil, err
			}
			i.Options = validatedOptions
		}

		// Add shipping to physical listings, and include it for digital and service
		// listings for legacy compatibility
		if contractType == pb.Listing_Metadata_PHYSICAL_GOOD.String() ||
			contractType == pb.Listing_Metadata_DIGITAL_GOOD.String() ||
			contractType == pb.Listing_Metadata_SERVICE.String() {

			i.ShippingOption = &pb.Order_Item_ShippingOption{
				Name:    item.Shipping.Name,
				Service: item.Shipping.Service,
			}
		}

		if contractType == pb.Listing_Metadata_CRYPTOCURRENCY.String() {
			i.PaymentAddress = item.PaymentAddress
			err = validateCryptocurrencyOrderItem(i)
			if err != nil {
				return nil, err
			}
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

func dedupeCoupons(itemCoupons []string) []string {
	couponMap := make(map[string]bool)
	var coupons []string
	for _, c := range itemCoupons {
		if !couponMap[c] {
			couponMap[c] = true
			coupons = append(coupons, c)
		}
	}
	return coupons
}

func getSignedListing(n *OpenBazaarNode, contract *pb.RicardianContract, item repo.Item) (*repo.Listing, error) {
	// Let's fetch the listing, should be cached
	b, err := ipfs.Cat(n.IpfsNode, item.ListingHash, time.Minute)
	if err != nil {
		return nil, err
	}

	//err = jsonpb.UnmarshalString(string(b), sl)
	sl, err := repo.UnmarshalJSONSignedListing(b)
	if err != nil {
		return nil, err
	}
	if err := validateVersionNumber(&sl.RListing); err != nil {
		return nil, err
	}
	if err := validateVendorID(&sl.RListing); err != nil {
		return nil, err
	}
	if err := repo.ValidateListing(&sl.RListing, n.TestNetworkEnabled() || n.RegressionNetworkEnabled()); err != nil {
		return nil, fmt.Errorf("listing failed to validate, reason: %q", err.Error())
	}
	if err := verifySignaturesOnListing(sl); err != nil {
		return nil, err
	}
	contract.VendorListings = append(contract.VendorListings, sl.RListing.ProtoListing)
	s := new(pb.Signature)
	s.Section = pb.Signature_LISTING
	s.SignatureBytes = sl.Signature
	contract.Signatures = append(contract.Signatures, s)
	return &sl.RListing, nil
}

func getRatingKeysForOrder(data *repo.PurchaseData, n *OpenBazaarNode, ts *timestamp.Timestamp) ([][]byte, error) {
	var ratingKeys [][]byte
	for range data.Items {
		// FIXME: bug here. This should use a different key for each item. This code doesn't look like it will do that.
		// Also the fix for this will also need to be included in the rating signing code.
		mPubkey, err := n.MasterPrivateKey.Neuter()
		if err != nil {
			return nil, err
		}
		ratingKey, err := mPubkey.Child(uint32(ts.Seconds))
		if err != nil {
			return nil, err
		}
		ecRatingKey, err := ratingKey.ECPubKey()
		if err != nil {
			return nil, err
		}
		ratingKeys = append(ratingKeys, ecRatingKey.SerializeCompressed())
	}
	return ratingKeys, nil
}

func getContractIdentity(n *OpenBazaarNode) (pb.ID, error) {
	id := pb.ID{}
	profile, err := n.GetProfile()
	if err == nil {
		id.Handle = profile.Handle
	}

	id.PeerID = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return id, err
	}
	keys := new(pb.ID_Pubkeys)
	keys.Identity = pubkey
	ecPubKey, err := n.MasterPrivateKey.ECPubKey()
	if err != nil {
		return id, err
	}
	keys.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = keys
	// Sign the PeerID with the Bitcoin key
	ecPrivKey, err := n.MasterPrivateKey.ECPrivKey()
	if err != nil {
		return id, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.PeerID))
	if err != nil {
		return id, err
	}
	id.BitcoinSig = sig.Serialize()
	return id, nil
}

func (n *OpenBazaarNode) currencyInAcceptedCurrenciesList(checkCode string, acceptedCurrencies []string) bool {
	checkDef, err := n.LookupCurrency(checkCode)
	if err != nil {
		return false
	}
	for _, cc := range acceptedCurrencies {
		acceptedDef, err := n.LookupCurrency(cc)
		if err != nil {
			continue
		}
		if checkDef.Equal(acceptedDef) {
			return true
		}
	}
	return false
}

func containsPhysicalGood(addedListings map[string]*repo.Listing) bool {
	for _, listing := range addedListings {
		contractType, err := listing.GetContractType()
		if err != nil {
			return false
		}
		if contractType == pb.Listing_Metadata_PHYSICAL_GOOD.String() {
			return true
		}
	}
	return false
}

func validatePhysicalPurchaseOrder(contract *pb.RicardianContract) error {
	if contract.BuyerOrder.Shipping == nil {
		return errors.New("order is missing shipping object")
	}
	if contract.BuyerOrder.Shipping.Address == "" {
		return errors.New("shipping address is empty")
	}
	if contract.BuyerOrder.Shipping.ShipTo == "" {
		return errors.New("ship to name is empty")
	}

	return nil
}

func validateCryptocurrencyOrderItem(item *pb.Order_Item) error {
	if len(item.Options) > 0 {
		return repo.ErrCryptocurrencyPurchaseIllegalField("item.options")
	}
	if len(item.CouponCodes) > 0 {
		return repo.ErrCryptocurrencyPurchaseIllegalField("item.couponCodes")
	}
	if item.PaymentAddress == "" {
		return ErrCryptocurrencyPurchasePaymentAddressRequired
	}
	if len(item.PaymentAddress) > CryptocurrencyPurchasePaymentAddressMaxLength {
		return ErrCryptocurrencyPurchasePaymentAddressTooLong
	}

	return nil
}

// EstimateOrderTotal - returns order total in satoshi/wei
func (n *OpenBazaarNode) EstimateOrderTotal(data *repo.PurchaseData) (*big.Int, error) {
	contract, err := n.createContractWithOrder(data)
	if err != nil {
		return big.NewInt(0), err
	}
	payment := new(pb.Order_Payment)
	defn, err := n.LookupCurrency(data.PaymentCoin)
	if err != nil {
		return big.NewInt(0), errors.New("invalid payment coin")
	}
	payment.AmountCurrency = &pb.CurrencyDefinition{
		Code:         defn.Code.String(),
		Divisibility: uint32(defn.Divisibility),
	}
	contract.BuyerOrder.Payment = payment
	return n.CalculateOrderTotal(contract)
}

// CancelOfflineOrder - cancel order
func (n *OpenBazaarNode) CancelOfflineOrder(contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return err
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	// Sweep the temp address into our wallet
	var utxos []wallet.TransactionInput
	for _, r := range records {
		if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
			addr, err := wal.DecodeAddress(r.Address)
			if err != nil {
				return err
			}
			outpointHash, err := hex.DecodeString(strings.TrimPrefix(r.Txid, "0x"))
			if err != nil {
				return fmt.Errorf("decoding transaction hash: %s", err.Error())
			}
			u := wallet.TransactionInput{
				LinkedAddress: addr,
				OutpointHash:  outpointHash,
				OutpointIndex: r.Index,
				Value:         r.Value,
			}
			utxos = append(utxos, u)
		}
	}

	if len(utxos) == 0 {
		return errors.New("cannot cancel order because utxo has already been spent")
	}

	chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
	if err != nil {
		return err
	}
	mECKey, err := n.MasterPrivateKey.ECPrivKey()
	if err != nil {
		return err
	}
	buyerKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
	if err != nil {
		return err
	}
	redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
	if err != nil {
		return err
	}
	refundAddress, err := wal.DecodeAddress(contract.BuyerOrder.RefundAddress)
	if err != nil {
		return err
	}
	_, err = wal.SweepAddress(utxos, &refundAddress, buyerKey, &redeemScript, wallet.NORMAL)
	if err != nil {
		return err
	}
	err = n.SendCancel(contract.VendorListings[0].VendorID.PeerID, orderID)
	if err != nil {
		return err
	}
	err = n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_CANCELED, true)
	if err != nil {
		log.Error(err)
	}
	return nil
}

// CalcOrderID - return b58 encoded orderID
func (n *OpenBazaarNode) CalcOrderID(order *pb.Order) (string, error) {
	ser, err := proto.Marshal(order)
	if err != nil {
		return "", err
	}
	id, err := ipfs.EncodeMultihash(ser)
	if err != nil {
		return "", err
	}
	return id.B58String(), nil
}

// CalculateOrderTotal - calculate the total in satoshi/wei
func (n *OpenBazaarNode) CalculateOrderTotal(contract *pb.RicardianContract) (*big.Int, error) {
	wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.AmountCurrency.Code)
	if err != nil {
		return big.NewInt(0), err
	}
	if wal.ExchangeRates() != nil {
		_, err = wal.ExchangeRates().GetLatestRate("") // Refresh the exchange rates
		if err != nil {
			log.Error(err)
		}
	}

	var total *big.Int
	physicalGoods := make(map[string]*pb.Listing)

	// Calculate the price of each item
	for _, item := range contract.BuyerOrder.Items {
		var (
			satoshis  *big.Int
			itemTotal *big.Int
		)

		l, err := ParseContractForListing(item.ListingHash, contract)
		if err != nil {
			return big.NewInt(0), fmt.Errorf("listing not found in contract for item %s", item.ListingHash)
		}
		rl, err := repo.NewListingFromProtobuf(l)
		if err != nil {
			return big.NewInt(0), err
		}

		if l.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			physicalGoods[item.ListingHash] = l
		}

		if l.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY &&
			l.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
			var priceModifier float64
			if l.Metadata.PriceModifier != 0 {
				priceModifier = float64(l.Metadata.PriceModifier)
			} else if l.Item.PriceModifier != 0 {
				priceModifier = float64(l.Item.PriceModifier)
			}
			satoshis, err = n.getPriceInSatoshi(contract.BuyerOrder.Payment.AmountCurrency.Code, l.Metadata.CryptoCurrencyCode, GetOrderQuantity(l, item), true)
			if err != nil {
				return big.NewInt(0), err
			}
			t0 := new(big.Float).Mul(big.NewFloat(priceModifier), new(big.Float).SetInt(satoshis))
			t1, _ := new(big.Float).Mul(t0, big.NewFloat(0.01)).Int(nil)
			satoshis = new(big.Int).Add(satoshis, t1)
		} else {
			p, ok := new(big.Int).SetString(l.Item.BigPrice, 10)
			if !ok {
				return big.NewInt(0), errors.New("invalid price value")
			}
			satoshis, err = n.getPriceInSatoshi(contract.BuyerOrder.Payment.AmountCurrency.Code, l.Item.PriceCurrency.Code, p, false)
			if err != nil {
				return big.NewInt(0), err
			}
		}
		itemTotal = new(big.Int).Set(satoshis)
		selectedSku, err := GetSelectedSku(l, item.Options)
		if err != nil {
			return big.NewInt(0), err
		}
		var skuExists bool
		skus, err := rl.GetSkus()
		if err != nil {
			return big.NewInt(0), err
		}
		for i, sku := range skus {
			if selectedSku == i {
				skuExists = true
				surcharge := big.NewInt(0)
				surcharge0, ok := new(big.Int).SetString(sku.BigSurcharge, 10)
				if ok {
					surcharge = new(big.Int).Abs(surcharge0)
				}
				if surcharge.Cmp(big.NewInt(0)) != 0 {
					satoshis, err := n.getPriceInSatoshi(contract.BuyerOrder.Payment.AmountCurrency.Code,
						l.Item.PriceCurrency.Code, surcharge, false)
					if err != nil {
						return big.NewInt(0), err
					}
					if surcharge0.Cmp(big.NewInt(0)) < 0 {
						itemTotal = new(big.Int).Sub(itemTotal, satoshis)
					} else {
						itemTotal = new(big.Int).Add(itemTotal, satoshis)
					}
				}
				if !skuExists {
					return big.NewInt(0), errors.New("selected variant not found in listing")
				}
				break
			}
		}
		// Subtract any coupons
		for _, couponCode := range item.CouponCodes {
			for _, vendorCoupon := range l.Coupons {
				id, err := ipfs.EncodeMultihash([]byte(couponCode))
				if err != nil {
					return big.NewInt(0), err
				}
				if id.B58String() == vendorCoupon.GetHash() {
					if d, ok := new(big.Int).SetString(vendorCoupon.BigPriceDiscount, 10); ok && d.Cmp(big.NewInt(0)) > 0 {
						satoshis, err := n.getPriceInSatoshi(contract.BuyerOrder.Payment.AmountCurrency.Code,
							l.Item.PriceCurrency.Code, d, false)
						if err != nil {
							log.Errorf("failed to convert currency for coupon (%s): %s", couponCode, err)
							continue
						}
						itemTotal = new(big.Int).Sub(itemTotal, satoshis)
					} else if discountF := vendorCoupon.GetPercentDiscount(); discountF > 0 {
						d := new(big.Float).Mul(big.NewFloat(float64(discountF)), big.NewFloat(0.01))
						totalDiscount, _ := new(big.Float).Mul(d, new(big.Float).SetInt(itemTotal)).Int(nil)
						itemTotal = new(big.Int).Sub(itemTotal, totalDiscount)
					}
				}
			}
		}
		// Apply tax
		for _, tax := range l.Taxes {
			for _, taxRegion := range tax.TaxRegions {
				if contract.BuyerOrder.Shipping.Country == taxRegion {
					t := new(big.Float).Mul(big.NewFloat(float64(tax.Percentage)), big.NewFloat(0.01))
					totalTax, _ := new(big.Float).Mul(t, new(big.Float).SetInt(itemTotal)).Int(nil)
					itemTotal = new(big.Int).Add(itemTotal, totalTax)
					break
				}
			}
		}

		if !(l.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY &&
			l.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE) {
			itemQuantity := GetOrderQuantity(l, item)
			if itemQuantity.Cmp(big.NewInt(0)) <= 0 {
				itemQuantity = big.NewInt(1)
				log.Debugf("missing quantity for order, assuming quantity 1")
			}
			itemTotal = new(big.Int).Mul(itemTotal, itemQuantity)
		}

		total = new(big.Int).Set(itemTotal)
	}

	shippingTotal, err := n.calculateShippingTotalForListings(contract, physicalGoods)
	if err != nil {
		return big.NewInt(0), err
	}
	total = new(big.Int).Add(total, shippingTotal)

	return total, nil
}

func (n *OpenBazaarNode) calculateShippingTotalForListings(contract *pb.RicardianContract, listings map[string]*pb.Listing) (*big.Int, error) {
	type itemShipping struct {
		primary               *big.Int
		secondary             *big.Int
		quantity              uint64
		shippingTaxPercentage float32
		version               uint32
	}
	var (
		is            []itemShipping
		shippingTotal *big.Int
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
			return big.NewInt(0), errors.New("shipping option not found in listing")
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
			return big.NewInt(0), errors.New("listing does ship to selected country")
		}

		// Check service exists
		services := make(map[string]*pb.Listing_ShippingOption_Service)
		for _, shippingService := range option.Services {
			services[strings.ToLower(shippingService.Name)] = shippingService
		}
		service, ok := services[strings.ToLower(item.ShippingOption.Service)]
		if !ok {
			return big.NewInt(0), errors.New("shipping service not found in listing")
		}
		servicePrice, ok := new(big.Int).SetString(service.BigPrice, 10)
		if !ok {
			return big.NewInt(0), errors.New("invalid service price")
		}
		shippingSatoshi, err := n.getPriceInSatoshi(contract.BuyerOrder.Payment.AmountCurrency.Code,
			listing.Item.PriceCurrency.Code, servicePrice, false)
		if err != nil {
			return big.NewInt(0), err
		}

		secondarySatoshi := big.NewInt(0)
		if service.BigAdditionalItemPrice != "" {
			serviceAddlItemPrice, ok := new(big.Int).SetString(service.BigAdditionalItemPrice, 10)
			if !ok {
				return big.NewInt(0), errors.New("invalid service additional price")
			}

			if serviceAddlItemPrice.Cmp(big.NewInt(0)) > 0 {
				secondarySatoshi, err = n.getPriceInSatoshi(contract.BuyerOrder.Payment.AmountCurrency.Code,
					listing.Item.PriceCurrency.Code, serviceAddlItemPrice, false)
				if err != nil {
					return big.NewInt(0), err
				}
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

		var qty uint64
		if q := quantityForItem(listing.Metadata.Version, item); q.IsUint64() {
			qty = q.Uint64()
		}
		is = append(is, itemShipping{
			primary:               shippingSatoshi,
			secondary:             secondarySatoshi,
			quantity:              qty,
			shippingTaxPercentage: shippingTaxPercentage,
			version:               listing.Metadata.Version,
		})
	}

	if len(is) == 0 {
		return big.NewInt(0), nil
	}

	if len(is) == 1 {
		s := int64(((1 + is[0].shippingTaxPercentage) * 100) + .5)
		shippingTotalPrimary := new(big.Int).Mul(is[0].primary, big.NewInt(s))
		stp, _ := new(big.Float).Mul(big.NewFloat(0.01), new(big.Float).SetInt(shippingTotalPrimary)).Int(nil)
		shippingTotal = stp
		if is[0].quantity > 1 {
			if is[0].version == 1 {
				t1 := new(big.Int).Mul(stp, big.NewInt(int64(is[0].quantity-1)))
				shippingTotal = new(big.Int).Add(stp, t1)
			} else if is[0].version >= 2 {
				shippingTotalSecondary := new(big.Int).Mul(is[0].secondary, big.NewInt(s))
				sts, _ := new(big.Float).Mul(big.NewFloat(0.01), new(big.Float).SetInt(shippingTotalSecondary)).Int(nil)

				t1 := new(big.Int).Mul(sts, big.NewInt(int64(is[0].quantity-1)))
				shippingTotal = new(big.Int).Add(stp, t1)

			} else {
				return big.NewInt(0), errors.New("unknown listing version")
			}
		}
		return shippingTotal, nil
	}

	var highest *big.Int
	var i int
	for x, s := range is {
		if s.primary.Cmp(highest) > 0 {
			highest = new(big.Int).Set(s.primary)
			i = x
		}
		s0 := int64(((1 + s.shippingTaxPercentage) * 100) + .5)
		shippingTotalSec := new(big.Int).Mul(s.secondary, big.NewInt(s0))
		sts0, _ := new(big.Float).Mul(big.NewFloat(0.01), new(big.Float).SetInt(shippingTotalSec)).Int(nil)
		shippingTotal0 := new(big.Int).Mul(sts0, big.NewInt(int64(s.quantity)))
		shippingTotal = new(big.Int).Add(shippingTotal, shippingTotal0)
	}
	sp := int64(((1 + is[i].shippingTaxPercentage) * 100) + .5)
	shippingTotalPrimary0 := new(big.Int).Mul(is[i].primary, big.NewInt(sp))
	stp0, _ := new(big.Float).Mul(big.NewFloat(0.01), new(big.Float).SetInt(shippingTotalPrimary0)).Int(nil)
	shippingTotal = new(big.Int).Sub(shippingTotal, stp0)

	shippingTotalSecondary0 := new(big.Int).Mul(is[i].secondary, big.NewInt(sp))
	sts0, _ := new(big.Float).Mul(big.NewFloat(0.01), new(big.Float).SetInt(shippingTotalSecondary0)).Int(nil)
	shippingTotal = new(big.Int).Add(shippingTotal, sts0)

	return shippingTotal, nil
}

func (n *OpenBazaarNode) getPriceInSatoshi(paymentCoin, originCurrencyCode string, amount *big.Int, isCryptoListing bool) (*big.Int, error) {
	var reserveCurrency = n.reserveCurrency()
	var (
		originCurrencyDef, oErr  = n.LookupCurrency(originCurrencyCode)
		paymentCurrencyDef, pErr = n.LookupCurrency(paymentCoin)
	)
	if oErr != nil {
		if isCryptoListing {
			// if the currency is unknown assume the default divisibility
			originCurrencyDef = repo.NewUnknownCryptoDefinition(paymentCoin, repo.DefaultCryptoDivisibility)
		} else {
			// Return error if not a valid listing currency code
			return big.NewInt(0), fmt.Errorf("invalid listing currency code: %s", oErr.Error())
		}

	}
	if pErr != nil {
		return big.NewInt(0), fmt.Errorf("invalid payment currency code: %s", pErr.Error())
	}

	// No conversion is necessary
	if originCurrencyDef.Equal(paymentCurrencyDef) {
		return amount, nil
	}

	reserveWallet, err := n.Multiwallet.WalletForCurrencyCode(reserveCurrency)
	if err != nil {
		return big.NewInt(0), fmt.Errorf("%s wallet not found for exchange rates", reserveCurrency)
	}

	converter := repo.NewCurrencyConverter(reserveWallet)

	finalPrice, err := converter.GetFinalPrice(originCurrencyDef, paymentCurrencyDef, amount)
	if err != nil {
		return nil, err
	}

	// Convert to satoshis
	divisibility := new(big.Int).SetUint64(uint64(paymentCurrencyDef.Divisibility))
	divisibilityFactor := new(big.Int).Exp(new(big.Int).SetInt64(10), divisibility, nil)
	finalPrice.Mul(finalPrice, new(big.Float).SetInt(divisibilityFactor))
	finalPriceInt, _ := finalPrice.Int(new(big.Int))

	return finalPriceInt, nil
}

func (n *OpenBazaarNode) reserveCurrency() string {
	if n.TestnetEnable || n.RegressionTestEnable {
		return "TBTC"
	}
	return "BTC"
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
			return errors.New("contract does not contain a signature for the order")
		case invalidSigError:
			return errors.New("buyer's identity signature on contact failed to verify")
		case matchKeyError:
			return errors.New("public key in order does not match reported buyer ID")
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
			return errors.New("buyer's bitcoin signature on GUID failed to verify")
		default:
			return err
		}
	}
	return nil
}

// ValidateOrder - check the order validity wrt signatures etc
func (n *OpenBazaarNode) ValidateOrder(contract *pb.RicardianContract, checkInventory bool) error {
	listingMap := make(map[string]*pb.Listing)

	// Check order contains all required fields
	if contract.BuyerOrder == nil {
		return errors.New("contract doesn't contain an order")
	}
	if contract.BuyerOrder.Payment == nil {
		return errors.New("order doesn't contain a payment")
	}
	if contract.BuyerOrder.BuyerID == nil {
		return errors.New("order doesn't contain a buyer ID")
	}
	if len(contract.BuyerOrder.Items) == 0 {
		return errors.New("order hasn't selected any items")
	}
	if len(contract.BuyerOrder.RatingKeys) != len(contract.BuyerOrder.Items) {
		return errors.New("number of rating keys do not match number of items")
	}
	for _, ratingKey := range contract.BuyerOrder.RatingKeys {
		if len(ratingKey) != 33 {
			return errors.New("invalid rating key in order")
		}
	}

	if !n.currencyInAcceptedCurrenciesList(contract.BuyerOrder.Payment.AmountCurrency.Code,
		contract.VendorListings[0].Metadata.AcceptedCurrencies) {
		return errors.New("payment coin not accepted")
	}

	if contract.BuyerOrder.Timestamp == nil {
		return errors.New("order is missing a timestamp")
	}
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		_, err := mh.FromB58String(contract.BuyerOrder.Payment.Moderator)
		if err != nil {
			return errors.New("invalid moderator")
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
			return errors.New("invalid moderator")
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
		listingID, err := ipfs.EncodeCID(ser)
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
		return errors.New("item hashes in the order do not match the included listings")
	}

	// Validate no duplicate coupons
	for _, item := range contract.BuyerOrder.Items {
		couponMap := make(map[string]bool)
		for _, c := range item.CouponCodes {
			if couponMap[c] {
				return errors.New("duplicate coupon code in order")
			}
			couponMap[c] = true
		}
	}

	// Validate the selected variants
	type inventory struct {
		Slug    string
		Variant int
		Count   *big.Int
	}
	var inventoryList []inventory
	for _, item := range contract.BuyerOrder.Items {
		var userOptions []*pb.Order_Item_Option
		var listingOptions []string
		for _, opt := range listingMap[item.ListingHash].Item.Options {
			listingOptions = append(listingOptions, opt.Name)
		}
		userOptions = append(userOptions, item.Options...)
		inv := inventory{Slug: listingMap[item.ListingHash].Slug}
		selectedVariant, err := GetSelectedSku(listingMap[item.ListingHash], item.Options)
		if err != nil {
			return err
		}
		inv.Variant = selectedVariant
		for _, o := range listingMap[item.ListingHash].Item.Options {
			for _, checkOpt := range userOptions {
				if strings.EqualFold(o.Name, checkOpt.Name) {
					validVariant := false
					for _, v := range o.Variants {
						if strings.EqualFold(v.Name, checkOpt.Value) {
							validVariant = true
						}
					}
					if !validVariant {
						return errors.New("selected variant not in listing")
					}
				}
			check:
				for i, lopt := range listingOptions {
					if strings.EqualFold(checkOpt.Name, lopt) {
						listingOptions = append(listingOptions[:i], listingOptions[i+1:]...)
						continue check
					}
				}
			}
		}
		if len(listingOptions) > 0 {
			return errors.New("not all options were selected")
		}
		// Create inventory paths to check later
		if q := GetOrderQuantity(listingMap[item.ListingHash], item); q.IsInt64() {
			inv.Count = q
		} else {
			// TODO: https://github.com/OpenBazaar/openbazaar-go/issues/1739
			return errors.New("big inventory quantity not supported")
		}
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
					return errors.New("shipping option not found in listing")
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
					return errors.New("listing does ship to selected country")
				}

				// Check service exists
				if option.Type != pb.Listing_ShippingOption_LOCAL_PICKUP {
					var service *pb.Listing_ShippingOption_Service
					for _, shippingService := range option.Services {
						if strings.EqualFold(shippingService.Name, item.ShippingOption.Service) {
							service = shippingService
						}
					}
					if service == nil {
						return errors.New("shipping service not found in listing")
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
				return errors.New("vendor has no inventory for the selected variant")
			}
			if amt.Cmp(big.NewInt(0)) >= 0 && amt.Cmp(inv.Count) < 0 {
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
			return errors.New("order is missing shipping object")
		}
		if contract.BuyerOrder.Shipping.Address == "" {
			return errors.New("shipping address is empty")
		}
		if contract.BuyerOrder.Shipping.ShipTo == "" {
			return errors.New("ship to name is empty")
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

// ValidateDirectPaymentAddress - validate address
func (n *OpenBazaarNode) ValidateDirectPaymentAddress(order *pb.Order) error {
	chaincode, err := hex.DecodeString(order.Payment.Chaincode)
	if err != nil {
		return err
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	mECKey, err := n.MasterPrivateKey.ECPubKey()
	if err != nil {
		return err
	}
	vendorKey, err := wal.ChildKey(mECKey.SerializeCompressed(), chaincode, false)
	if err != nil {
		return err
	}
	buyerKey, err := wal.ChildKey(order.BuyerID.Pubkeys.Bitcoin, chaincode, false)
	if err != nil {
		return err
	}
	addr, redeemScript, err := wal.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey}, 1, time.Duration(0), nil)
	if err != nil {
		return err
	}

	if order.Payment.Address != addr.String() {
		return errors.New("invalid payment address")
	}
	if order.Payment.RedeemScript != hex.EncodeToString(redeemScript) {
		return errors.New("invalid redeem script")
	}
	return nil
}

// ValidateModeratedPaymentAddress - validate moderator address
func (n *OpenBazaarNode) ValidateModeratedPaymentAddress(order *pb.Order, timeout time.Duration) error {
	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	ipnsPath := ipfspath.FromString(order.Payment.Moderator + "/profile.json")
	profileBytes, err := ipfs.ResolveThenCat(n.IpfsNode, ipnsPath, time.Minute, n.IPNSQuorumSize, true)
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
	mECKey, err := n.MasterPrivateKey.ECPubKey()
	if err != nil {
		return err
	}
	vendorKey, err := wal.ChildKey(mECKey.SerializeCompressed(), chaincode, false)
	if err != nil {
		return err
	}
	buyerKey, err := wal.ChildKey(order.BuyerID.Pubkeys.Bitcoin, chaincode, false)
	if err != nil {
		return err
	}
	moderatorKey, err := wal.ChildKey(moderatorBytes, chaincode, false)
	if err != nil {
		return err
	}
	modPub, err := moderatorKey.ECPubKey()
	if err != nil {
		return err
	}
	if !bytes.Equal(order.Payment.ModeratorKey, modPub.SerializeCompressed()) {
		return errors.New("invalid moderator key")
	}
	addr, redeemScript, err := wal.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2, timeout, vendorKey)
	if err != nil {
		return err
	}
	if strings.TrimPrefix(order.Payment.Address, "0x") != strings.TrimPrefix(addr.String(), "0x") {
		return errors.New("invalid payment address")
	}
	if order.Payment.RedeemScript != hex.EncodeToString(redeemScript) {
		return errors.New("invalid redeem script")
	}
	return nil
}

// SignOrder - add signature to the order
func (n *OpenBazaarNode) SignOrder(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrder, err := proto.Marshal(contract.BuyerOrder)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_ORDER
	idSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrder)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = idSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func validateVendorID(l *repo.Listing) error {

	listing := l.ProtoListing

	if listing == nil {
		return errors.New("listing is nil")
	}
	if listing.VendorID == nil {
		return errors.New("vendorID is nil")
	}
	if listing.VendorID.Pubkeys == nil {
		return errors.New("vendor pubkeys is nil")
	}
	vendorPubKey, err := crypto.UnmarshalPublicKey(listing.VendorID.Pubkeys.Identity)
	if err != nil {
		return err
	}
	vendorID, err := peer.IDB58Decode(listing.VendorID.PeerID)
	if err != nil {
		return err
	}
	if !vendorID.MatchesPublicKey(vendorPubKey) {
		return errors.New("invalid vendorID")
	}
	return nil
}

func validateVersionNumber(listing *repo.Listing) error {
	if listing == nil {
		return errors.New("listing is nil")
	}
	if listing.Metadata.Version > repo.ListingVersion {
		return errors.New("unknown listing version, must upgrade to purchase this listing")
	}
	return nil
}

// ValidatePaymentAmount - validate amount requested
func (n *OpenBazaarNode) ValidatePaymentAmount(requestedAmount, paymentAmount *big.Int) bool {
	settings, _ := n.Datastore.Settings().Get()
	bufferPercent := float32(0)
	if settings.MisPaymentBuffer != nil {
		bufferPercent = *settings.MisPaymentBuffer
	}
	a := new(big.Float).SetInt(requestedAmount)
	buf := new(big.Float).Mul(a, big.NewFloat(float64(bufferPercent)))
	buf = new(big.Float).Mul(buf, big.NewFloat(0.01))
	rh := new(big.Float).SetInt(paymentAmount)
	rh = new(big.Float).Add(rh, buf)
	return rh.Cmp(a) >= 0
}

// ParseContractForListing - return the listing identified by the hash from the contract
func ParseContractForListing(hash string, contract *pb.RicardianContract) (*pb.Listing, error) {
	for _, listing := range contract.VendorListings {
		ser, err := proto.Marshal(listing)
		if err != nil {
			return nil, err
		}
		listingID, err := ipfs.EncodeCID(ser)
		if err != nil {
			return nil, err
		}
		if hash == listingID.String() {
			return listing, nil
		}
	}
	return nil, errors.New("listing not found")
}

// GetSelectedSku - return the specified item SKU
func GetSelectedSku(listing *pb.Listing, itemOptions []*pb.Order_Item_Option) (int, error) {
	if len(itemOptions) == 0 && (len(listing.Item.Skus) == 1 || len(listing.Item.Skus) == 0) {
		// Default sku
		return 0, nil
	}
	var selected []int
	for _, s := range listing.Item.Options {
	optionsLoop:
		for _, o := range itemOptions {
			if strings.EqualFold(o.Name, s.Name) {
				for i, va := range s.Variants {
					if strings.EqualFold(va.Name, o.Value) {
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
	return 0, errors.New("no skus selected")
}

// SameSku - check if the variants have the same SKU
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

// GetOrderQuantity - return the specified item quantity
func GetOrderQuantity(l *pb.Listing, item *pb.Order_Item) *big.Int {
	return quantityForItem(l.Metadata.Version, item)
}

func quantityForItem(version uint32, item *pb.Order_Item) *big.Int {
	switch version {
	case 5:
		i, _ := new(big.Int).SetString(item.BigQuantity, 10)
		return i
	case 3, 4:
		return new(big.Int).SetUint64(item.Quantity64)
	}
	return new(big.Int).SetUint64(uint64(item.Quantity))
}
