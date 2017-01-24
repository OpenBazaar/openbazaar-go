package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	ipfspath "github.com/ipfs/go-ipfs/path"
	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	crypto "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
	"path"
	"strings"
	"time"
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
	ListingHash string         `json:"listingHash"`
	Quantity    int            `json:"quantity"`
	Options     []option       `json:"options"`
	Shipping    shippingOption `json:"shipping"`
	Memo        string         `json:"memo"`
	Coupons     []string       `json:"coupons"`
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

func (n *OpenBazaarNode) Purchase(data *PurchaseData) (orderId string, paymentAddress string, paymentAmount uint64, vendorOnline bool, err error) {
	contract := new(pb.RicardianContract)
	order := new(pb.Order)
	if data.RefundAddress != nil {
		order.RefundAddress = *(data.RefundAddress)
	} else {
		order.RefundAddress = n.Wallet.CurrentAddress(spvwallet.INTERNAL).EncodeAddress()
	}
	shipping := new(pb.Order_Shipping)
	shipping.ShipTo = data.ShipTo
	shipping.Address = data.Address
	shipping.City = data.City
	shipping.State = data.State
	shipping.PostalCode = data.PostalCode
	shipping.Country = pb.CountryCode(pb.CountryCode_value[data.CountryCode])
	order.Shipping = shipping

	id := new(pb.ID)
	profile, err := n.GetProfile()
	if err == nil {
		id.BlockchainID = profile.Handle
	}

	id.Guid = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return "", "", 0, false, err
	}
	keys := new(pb.ID_Pubkeys)
	keys.Guid = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return "", "", 0, false, err
	}
	keys.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = keys
	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return "", "", 0, false, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.Guid))
	id.BitcoinSig = sig.Serialize()
	order.BuyerID = id

	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	order.Timestamp = ts
	order.AlternateContactInfo = data.AlternateContactInfo

	var ratingKeys [][]byte
	for range data.Items {
		// FIXME: bug here. This should use a different key for each item. This code doesn't look like it will do that.
		// Also the fix for this will also need to be included in the rating signing code.
		ratingKey, err := n.Wallet.MasterPublicKey().Child(uint32(ts.Seconds))
		if err != nil {
			return "", "", 0, false, err
		}
		ecRatingKey, err := ratingKey.ECPubKey()
		if err != nil {
			return "", "", 0, false, err
		}
		ratingKeys = append(ratingKeys, ecRatingKey.SerializeCompressed())
	}
	order.RatingKeys = ratingKeys

	var addedListings [][]string
	for _, item := range data.Items {
		i := new(pb.Order_Item)

		/* It is possible that multiple items could refer to the same listing if the buyer is ordering
		   multiple items with different variants. If it is multiple items of the same variant they can just
		   use the quantity field. But different variants require two separate item entries. However,
		   in this case we do not need to add the listing to the contract twice. Just once is sufficient.
		   So let's check to see if that's the case here and handle it. */
		toAdd := true
		for _, addedListing := range addedListings {
			if item.ListingHash == addedListing[0] {
				toAdd = false
			}
		}
		listing := new(pb.Listing)
		if toAdd {
			// Let's fetch the listing, should be cached
			b, err := ipfs.Cat(n.Context, item.ListingHash)
			if err != nil {
				return "", "", 0, false, err
			}
			rc := new(pb.RicardianContract)
			err = jsonpb.UnmarshalString(string(b), rc)
			if err != nil {
				return "", "", 0, false, err
			}
			if err := validateVersionNumber(rc); err != nil {
				return "", "", 0, false, err
			}
			if err := validateVendorID(rc); err != nil {
				return "", "", 0, false, err
			}
			if err := validateListing(rc.VendorListings[0]); err != nil {
				return "", "", 0, false, fmt.Errorf("Listing failed to validate, reason: %q", err.Error())
			}
			if err := verifySignaturesOnListing(rc); err != nil {
				return "", "", 0, false, err
			}
			contract.VendorListings = append(contract.VendorListings, rc.VendorListings[0])
			contract.Signatures = append(contract.Signatures, rc.Signatures[0])
			addedListings = append(addedListings, []string{item.ListingHash, rc.VendorListings[0].Slug})
			listing = rc.VendorListings[0]
		} else {
			for _, addedListing := range addedListings {
				if addedListing[0] == item.ListingHash {
					for _, l := range contract.VendorListings {
						if l.Slug == addedListing[1] {
							listing = l
							break
						}
					}
				}
			}
		}

		if strings.ToLower(listing.Metadata.AcceptedCurrency) != strings.ToLower(n.Wallet.CurrencyCode()) {
			return "", "", 0, false, fmt.Errorf("Contract only accepts %s, our wallet uses %s", listing.Metadata.AcceptedCurrency, n.Wallet.CurrencyCode())
		}

		// Validate the selected options
		var userOptions []option
		var listingOptions []string
		for _, opt := range listing.Item.Options {
			listingOptions = append(listingOptions, opt.Name)
		}
		for _, uopt := range item.Options {
			userOptions = append(userOptions, uopt)
		}
		for _, checkOpt := range userOptions {
			for _, o := range listing.Item.Options {
				if strings.ToLower(o.Name) == strings.ToLower(checkOpt.Name) {
					var validVariant bool = false
					for _, v := range o.Variants {
						if strings.ToLower(v.Name) == strings.ToLower(checkOpt.Value) {
							validVariant = true
						}
					}
					if validVariant == false {
						return "", "", 0, false, errors.New("Selected variant not in listing")
					}
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
		if len(listingOptions) > 0 {
			return "", "", 0, false, errors.New("Not all options were selected")
		}

		ser, err := proto.Marshal(listing)
		if err != nil {
			return "", "", 0, false, err
		}
		h := sha256.Sum256(ser)
		encoded, err := mh.Encode(h[:], mh.SHA2_256)
		if err != nil {
			return "", "", 0, false, err
		}
		listingMH, err := mh.Cast(encoded)
		if err != nil {
			return "", "", 0, false, err
		}
		i.ListingHash = listingMH.B58String()
		i.Quantity = uint32(item.Quantity)

		for _, option := range item.Options {
			o := new(pb.Order_Item_Option)
			o.Name = option.Name
			o.Value = option.Value
			i.Options = append(i.Options, o)
		}
		so := new(pb.Order_Item_ShippingOption)
		so.Name = item.Shipping.Name
		so.Service = item.Shipping.Service
		i.ShippingOption = so
		i.Memo = item.Memo
		i.CouponCodes = item.Coupons
		order.Items = append(order.Items, i)
	}

	contract.BuyerOrder = order

	// Add payment data and send to vendor
	if data.Moderator != "" { // Moderated payment
		payment := new(pb.Order_Payment)
		payment.Method = pb.Order_Payment_MODERATED
		payment.Moderator = data.Moderator
		ipnsPath := ipfspath.FromString(data.Moderator + "/profile")
		profileBytes, err := ipfs.ResolveThenCat(n.Context, ipnsPath)
		if err != nil {
			return "", "", 0, false, err
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
		total, err := n.CalculateOrderTotal(contract)
		if err != nil {
			return "", "", 0, false, err
		}
		payment.Amount = total

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

		addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2)
		if err != nil {
			return "", "", 0, false, err
		}
		payment.Address = addr.EncodeAddress()
		payment.RedeemScript = hex.EncodeToString(redeemScript)
		payment.Chaincode = hex.EncodeToString(chaincode)
		contract.BuyerOrder.Payment = payment
		contract.BuyerOrder.RefundFee = n.Wallet.GetFeePerByte(spvwallet.NORMAL)

		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return "", "", 0, false, err
		}
		n.Wallet.AddWatchedScript(script)

		contract, err = n.SignOrder(contract)
		if err != nil {
			return "", "", 0, false, err
		}

		// Send to order vendor
		resp, err := n.SendOrder(contract.VendorListings[0].VendorID.Guid, contract)
		if err != nil { // Vendor offline
			// Send using offline messaging
			log.Warningf("Vendor %s is offline, sending offline order message", contract.VendorListings[0].VendorID.Guid)
			peerId, err := peer.IDB58Decode(contract.VendorListings[0].VendorID.Guid)
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
			k, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Guid)
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
			n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_PENDING, false)
			return orderId, contract.BuyerOrder.Payment.Address, contract.BuyerOrder.Payment.Amount, false, err
		} else { // Vendor responded
			if resp.MessageType == pb.Message_ERROR {
				return "", "", 0, false, fmt.Errorf("Vendor rejected order, reason: %s", string(resp.Payload.Value))
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
			n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_CONFIRMED, true)
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
		resp, err := n.SendOrder(contract.VendorListings[0].VendorID.Guid, contract)
		if err != nil { // Vendor offline
			// Change payment code to direct
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
			addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey}, 1)
			if err != nil {
				return "", "", 0, false, err
			}
			payment.Address = addr.EncodeAddress()
			payment.RedeemScript = hex.EncodeToString(redeemScript)
			payment.Chaincode = hex.EncodeToString(chaincode)

			script, err := txscript.PayToAddrScript(addr)
			if err != nil {
				return "", "", 0, false, err
			}
			n.Wallet.AddWatchedScript(script)

			contract, err = n.SignOrder(contract)
			if err != nil {
				return "", "", 0, false, err
			}

			// Send using offline messaging
			log.Warningf("Vendor %s is offline, sending offline order message", contract.VendorListings[0].VendorID.Guid)
			peerId, err := peer.IDB58Decode(contract.VendorListings[0].VendorID.Guid)
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
			k, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Guid)
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
			n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_PENDING, false)
			return orderId, contract.BuyerOrder.Payment.Address, contract.BuyerOrder.Payment.Amount, false, err
		} else { // Vendor responded
			if resp.MessageType == pb.Message_ERROR {
				return "", "", 0, false, fmt.Errorf("Vendor rejected order, reason: %s", string(resp.Payload.Value))
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
			orderId, err := n.CalcOrderId(contract.BuyerOrder)
			if err != nil {
				return "", "", 0, false, err
			}
			n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_CONFIRMED, true)
			return orderId, contract.VendorOrderConfirmation.PaymentAddress, contract.BuyerOrder.Payment.Amount, true, nil
		}
	}
}

func (n *OpenBazaarNode) CancelOfflineOrder(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}
	// Sweep the temp address into our wallet
	var utxos []spvwallet.Utxo
	for _, r := range records {
		if !r.Spent && r.Value > 0 {
			u := spvwallet.Utxo{}
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
	refundAddress, err := btcutil.DecodeAddress(contract.BuyerOrder.RefundAddress, n.Wallet.Params())
	if err != nil {
		return err
	}
	err = n.Wallet.SweepMultisig(utxos, &refundAddress, buyerKey, redeemScript, spvwallet.NORMAL)
	if err != nil {
		return err
	}
	err = n.SendCancel(contract.VendorListings[0].VendorID.Guid, orderId)
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
	orderBytes := sha256.Sum256(ser)
	encoded, err := mh.Encode(orderBytes[:], mh.SHA2_256)
	if err != nil {
		return "", err
	}
	multihash, err := mh.Cast(encoded)
	if err != nil {
		return "", err
	}
	return multihash.B58String(), nil
}

func (n *OpenBazaarNode) CalculateOrderTotal(contract *pb.RicardianContract) (uint64, error) {
	if n.ExchangeRates != nil {
		n.ExchangeRates.GetLatestRate("") // Refresh the exchange rates
	}
	var total uint64
	physicalGoods := make(map[string]*pb.Listing)

	// Calculate the price of each item
	for _, item := range contract.BuyerOrder.Items {
		var itemTotal uint64
		var l *pb.Listing
		for _, listing := range contract.VendorListings {
			ser, err := proto.Marshal(listing)
			if err != nil {
				return 0, err
			}
			h := sha256.Sum256(ser)
			encoded, err := mh.Encode(h[:], mh.SHA2_256)
			if err != nil {
				return 0, err
			}
			listingMH, err := mh.Cast(encoded)
			if err != nil {
				return 0, err
			}
			if item.ListingHash == listingMH.B58String() {
				l = listing
				break
			}
		}
		if l == nil {
			return 0, fmt.Errorf("Listing not found in contract for item %s", item.ListingHash)
		}
		if l.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			physicalGoods[item.ListingHash] = l
		}
		satoshis, err := n.getPriceInSatoshi(l.Metadata.PricingCurrency, l.Item.Price)
		if err != nil {
			return 0, err
		}
		itemTotal += satoshis
		for _, option := range item.Options {
			optionExists := false
			for _, listingOption := range l.Item.Options {
				if strings.ToLower(option.Name) == strings.ToLower(listingOption.Name) {
					optionExists = true
					variantExists := false
					for _, variant := range listingOption.Variants {
						if strings.ToLower(variant.Name) == strings.ToLower(option.Value) {
							if variant.PriceModifier > 0 {
								satoshis, err := n.getPriceInSatoshi(l.Metadata.PricingCurrency, uint64(variant.PriceModifier))
								if variant.PriceModifier < 0 {
									satoshis = -satoshis
								}
								if err != nil {
									return 0, err
								}
								itemTotal += satoshis
							}
							variantExists = true
							break
						}
					}
					if !variantExists {
						return 0, errors.New("Selected variant not found in listing")
					}
					break
				}
			}
			if !optionExists {
				return 0, errors.New("Selected option not found in listing")
			}
		}
		// Subtract any coupons
		for _, couponCode := range item.CouponCodes {
			for _, vendorCoupon := range l.Coupons {
				h := sha256.Sum256([]byte(couponCode))
				encoded, err := mh.Encode(h[:], mh.SHA2_256)
				if err != nil {
					return 0, err
				}
				multihash, err := mh.Cast(encoded)
				if err != nil {
					return 0, err
				}
				if multihash.B58String() == vendorCoupon.Hash {
					if discount := vendorCoupon.GetPriceDiscount(); discount > 0 {
						itemTotal -= discount
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
				}
			}
		}
		itemTotal *= uint64(item.Quantity)
		total += itemTotal
	}

	// Add in shipping costs
	type combinedShipping struct {
		quantity int
		price    uint64
		add      bool
		modifier uint64
	}
	var combinedOptions []combinedShipping

	var shippingTotal uint64
	for listingHash, listing := range physicalGoods {
		for _, item := range contract.BuyerOrder.Items {
			if item.ListingHash == listingHash {
				var itemShipping uint64
				// Check selected option exists
				var option *pb.Listing_ShippingOption
				for _, shippingOption := range listing.ShippingOptions {
					if shippingOption.Name == item.ShippingOption.Name {
						option = shippingOption
						break
					}
				}
				if option == nil {
					return 0, errors.New("Shipping option not found in listing")
				}

				// Check that this option ships to us
				shipsToMe := false
				for _, country := range option.Regions {
					if country == contract.BuyerOrder.Shipping.Country || country == pb.CountryCode_ALL {
						shipsToMe = true
						break
					}
				}
				if !shipsToMe {
					return 0, errors.New("Listing does ship to selected country")
				}

				// Check service exists
				var service *pb.Listing_ShippingOption_Service
				for _, shippingService := range option.Services {
					if strings.ToLower(shippingService.Name) == strings.ToLower(item.ShippingOption.Service) {
						service = shippingService
					}
				}
				if service == nil {
					return 0, errors.New("Shipping service not found in listing")
				}
				shippingSatoshi, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, service.Price)
				if err != nil {
					return 0, err
				}
				shippingPrice := uint64(item.Quantity) * shippingSatoshi
				itemShipping += shippingPrice
				shippingTaxPercentage := float32(0)

				// Calculate tax percentage
				for _, tax := range listing.Taxes {
					for _, taxRegion := range tax.TaxRegions {
						if contract.BuyerOrder.Shipping.Country == taxRegion && tax.TaxShipping {
							shippingTaxPercentage = tax.Percentage / 100
						}
					}
				}

				// Apply shipping rules
				if option.ShippingRules != nil {
					for _, rule := range option.ShippingRules.Rules {
						switch option.ShippingRules.RuleType {
						case pb.Listing_ShippingOption_ShippingRules_QUANTITY_DISCOUNT:
							if item.Quantity >= rule.MinRange && item.Quantity <= rule.MaxRange {
								rulePrice, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, rule.Price)
								if err != nil {
									return 0, err
								}
								itemShipping -= rulePrice
							}
						case pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_QUANTITY_RANGE:
							if item.Quantity >= rule.MinRange && item.Quantity <= rule.MaxRange {
								itemShipping -= shippingPrice
								rulePrice, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, rule.Price)
								if err != nil {
									return 0, err
								}
								itemShipping += rulePrice
							}
						case pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_WEIGHT_RANGE:
							weight := listing.Item.Grams * float32(item.Quantity)
							if uint32(weight) >= rule.MinRange && uint32(weight) <= rule.MaxRange {
								itemShipping -= shippingPrice
								rulePrice, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, rule.Price)
								if err != nil {
									return 0, err
								}
								itemShipping += rulePrice
							}
						case pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_ADD:
							itemShipping -= shippingPrice
							rulePrice, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, rule.Price)
							rulePrice += uint64(float32(rulePrice) * shippingTaxPercentage)
							shippingSatoshi += uint64(float32(shippingSatoshi) * shippingTaxPercentage)
							if err != nil {
								return 0, err
							}
							cs := combinedShipping{
								quantity: int(item.Quantity),
								price:    shippingSatoshi,
								add:      true,
								modifier: rulePrice,
							}
							combinedOptions = append(combinedOptions, cs)

						case pb.Listing_ShippingOption_ShippingRules_COMBINED_SHIPPING_SUBTRACT:
							itemShipping -= shippingPrice
							rulePrice, err := n.getPriceInSatoshi(listing.Metadata.PricingCurrency, rule.Price)
							rulePrice += uint64(float32(rulePrice) * shippingTaxPercentage)
							shippingSatoshi += uint64(float32(shippingSatoshi) * shippingTaxPercentage)
							if err != nil {
								return 0, err
							}
							cs := combinedShipping{
								quantity: int(item.Quantity),
								price:    shippingSatoshi,
								add:      false,
								modifier: rulePrice,
							}
							combinedOptions = append(combinedOptions, cs)
						}
					}
				}
				// Apply tax
				itemShipping += uint64(float32(itemShipping) * shippingTaxPercentage)
				shippingTotal += itemShipping
			}
		}
	}

	// Process combined shipping rules
	if len(combinedOptions) > 0 {
		lowestPrice := int64(-1)
		for _, v := range combinedOptions {
			if int64(v.price) < lowestPrice || lowestPrice == -1 {
				lowestPrice = int64(v.price)
			}
		}
		shippingTotal += uint64(lowestPrice)
		for _, o := range combinedOptions {
			modifier := o.modifier
			modifier *= (uint64(o.quantity) - 1)
			if o.add {
				shippingTotal += modifier
			} else {
				shippingTotal -= modifier
			}
		}
	}

	total += shippingTotal
	return total, nil
}

func (n *OpenBazaarNode) getPriceInSatoshi(currencyCode string, amount uint64) (uint64, error) {
	if strings.ToLower(currencyCode) == strings.ToLower(n.Wallet.CurrencyCode()) {
		return amount, nil
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

func verifySignaturesOnOrder(contract *pb.RicardianContract) error {
	if err := verifyMessageSignature(
		contract.BuyerOrder,
		contract.BuyerOrder.BuyerID.Pubkeys.Guid,
		contract.Signatures,
		pb.Signature_ORDER,
		contract.BuyerOrder.BuyerID.Guid,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the order")
		case invalidSigError:
			return errors.New("Buyer's guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in order does not match reported buyer ID")
		default:
			return err
		}
	}

	if err := verifyBitcoinSignature(
		contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
		contract.BuyerOrder.BuyerID.BitcoinSig,
		contract.BuyerOrder.BuyerID.Guid,
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

func (n *OpenBazaarNode) ValidateOrder(contract *pb.RicardianContract) error {
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
	for _, listing := range contract.VendorListings {
		ser, err := proto.Marshal(listing)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(ser)
		encoded, err := mh.Encode(hash[:], mh.SHA2_256)
		if err != nil {
			return err
		}
		multihash, err := mh.Cast(encoded)
		if err != nil {
			return err
		}
		for i, hash := range itemHashes {
			if hash == multihash.B58String() {
				itemHashes = append(itemHashes[:i], itemHashes[i+1:]...)
				listingMap[hash] = listing
			}
		}
	}
	if len(itemHashes) > 0 {
		return errors.New("Item hashes in the order do not match the included listings")
	}

	// Validate the each item in the order is for sale
	for _, listing := range contract.VendorListings {
		if !n.IsItemForSale(listing) {
			return errors.New("Contract contained item that is not for sale")
		}
	}

	// Validate the selected variants
	var inventory []map[string]int
	for _, item := range contract.BuyerOrder.Items {
		var userOptions []*pb.Order_Item_Option
		var listingOptions []string
		for _, opt := range listingMap[item.ListingHash].Item.Options {
			listingOptions = append(listingOptions, opt.Name)
		}
		for _, uopt := range item.Options {
			userOptions = append(userOptions, uopt)
		}
		inv := make(map[string]int)
		invPath := listingMap[item.ListingHash].Slug
		for _, o := range listingMap[item.ListingHash].Item.Options {
			for _, checkOpt := range userOptions {
				if strings.ToLower(o.Name) == strings.ToLower(checkOpt.Name) {
					var validVariant bool = false
					for _, v := range o.Variants {
						if strings.ToLower(v.Name) == strings.ToLower(checkOpt.Value) {
							validVariant = true
							invPath = path.Join(invPath, v.Name)
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
		inv[invPath] = int(item.Quantity)
		inventory = append(inventory, inv)
	}

	// Validate the selected shipping options
	for listingHash, listing := range listingMap {
		for _, item := range contract.BuyerOrder.Items {
			if item.ListingHash == listingHash {
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
		}
	}

	// Check we have enough inventory
	for _, invMap := range inventory {
		for invString, quantity := range invMap {
			amt, err := n.Datastore.Inventory().GetSpecific(invString)
			if err != nil {
				return errors.New("Vendor has no inventory for the selected variant.")
			}
			if amt >= 0 && amt < quantity {
				return fmt.Errorf("Not enough inventory for item %s, only %d in stock", invString, amt)
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
		if contract.BuyerOrder.Shipping.City == "" {
			return errors.New("Shipping city is empty")
		}
		if contract.BuyerOrder.Shipping.ShipTo == "" {
			return errors.New("Ship to name is empty")
		}
		if contract.BuyerOrder.Shipping.State == "" {
			return errors.New("Shipping state is empty")
		}
	}

	// Validate the buyers's signature on the order
	err := verifySignaturesOnOrder(contract)
	if err != nil {
		return err
	}
	return nil
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
	addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey}, 1)
	if order.Payment.Address != addr.EncodeAddress() {
		return errors.New("Invalid payment address")
	}
	if order.Payment.RedeemScript != hex.EncodeToString(redeemScript) {
		return errors.New("Invalid redeem script")
	}
	return nil
}

func (n *OpenBazaarNode) ValidateModeratedPaymentAddress(order *pb.Order) error {
	ipnsPath := ipfspath.FromString(order.Payment.Moderator + "/profile")
	profileBytes, err := ipfs.ResolveThenCat(n.Context, ipnsPath)
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
	addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *ModeratorKey}, 2)
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
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrder)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func validateVendorID(rc *pb.RicardianContract) error {

	if len(rc.VendorListings) == 0 {
		return errors.New("Contract does not contain a listing")
	}
	if rc.VendorListings[0].VendorID == nil {
		return errors.New("VendorID is nil")
	}
	if rc.VendorListings[0].VendorID.Pubkeys == nil {
		return errors.New("Vendor pubkeys is nil")
	}
	vendorPubKey, err := crypto.UnmarshalPublicKey(rc.VendorListings[0].VendorID.Pubkeys.Guid)
	if err != nil {
		return err
	}
	vendorId, err := peer.IDB58Decode(rc.VendorListings[0].VendorID.Guid)
	if err != nil {
		return err
	}
	if !vendorId.MatchesPublicKey(vendorPubKey) {
		return errors.New("Invalid vendor ID")
	}
	return nil
}

func validateVersionNumber(rc *pb.RicardianContract) error {
	if len(rc.VendorListings) == 0 {
		return errors.New("Contract does not contain a listing")
	}
	if rc.VendorListings[0].Metadata == nil {
		return errors.New("Contract does not contain listing metadata")
	}
	if rc.VendorListings[0].Metadata.Version > ListingVersion {
		return errors.New("Unkown listing version. You must upgrade to purchase this listing.")
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
