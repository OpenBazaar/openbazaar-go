package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/golang/protobuf/ptypes"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func (i *jsonAPIHandler) GETPurchases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	purchases, queryCount, err := i.node.Datastore.Purchases().GetAll(orderStates, searchTerm, sortByAscending, sortByRead, limit, []string{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, p := range purchases {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(p.OrderId)
		if err != nil {
			continue
		}
		purchases[n].UnreadChatMessages = unread
	}
	type purchasesResponse struct {
		QueryCount int             `json:"queryCount"`
		Purchases  []repo.Purchase `json:"purchases"`
	}
	pr := purchasesResponse{queryCount, purchases}
	ret, err := json.MarshalIndent(pr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) GETSales(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	sales, queryCount, err := i.node.Datastore.Sales().GetAll(orderStates, searchTerm, sortByAscending, sortByRead, limit, []string{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, s := range sales {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(s.OrderId)
		if err != nil {
			continue
		}
		sales[n].UnreadChatMessages = unread
	}
	type salesResponse struct {
		QueryCount int         `json:"queryCount"`
		Sales      []repo.Sale `json:"sales"`
	}
	sr := salesResponse{queryCount, sales}

	ret, err := json.MarshalIndent(sr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) GETCases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	cases, queryCount, err := i.node.Datastore.Cases().GetAll(orderStates, searchTerm, sortByAscending, sortByRead, limit, []string{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, c := range cases {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(c.CaseId)
		if err != nil {
			continue
		}
		cases[n].UnreadChatMessages = unread
	}
	type casesResponse struct {
		QueryCount int         `json:"queryCount"`
		Cases      []repo.Case `json:"cases"`
	}
	cr := casesResponse{queryCount, cases}
	ret, err := json.MarshalIndent(cr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTPurchases(w http.ResponseWriter, r *http.Request) {
	records, err := getTradeRecords(r, w, i.node, "purchases")
	if err != nil {
		return
	}
	SanitizedResponse(w, records)
}

func (i *jsonAPIHandler) POSTSales(w http.ResponseWriter, r *http.Request) {
	records, err := getTradeRecords(r, w, i.node, "sales")
	if err != nil {
		return
	}
	SanitizedResponse(w, records)
}

func (i *jsonAPIHandler) POSTCases(w http.ResponseWriter, r *http.Request) {
	records, err := getTradeRecords(r, w, i.node, "cases")
	if err != nil {
		return
	}
	SanitizedResponse(w, records)
}

func (i *jsonAPIHandler) POSTOrderFulfill(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var fulfill pb.OrderFulfillment
	err := decoder.Decode(&fulfill)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, err := i.node.Datastore.Sales().GetByOrderId(fulfill.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	if state != pb.OrderState_AWAITING_FULFILLMENT && state != pb.OrderState_PARTIALLY_FULFILLED {
		ErrorResponse(w, http.StatusBadRequest, "order must be in state AWAITING_FULFILLMENT or PARTIALLY_FULFILLED to fulfill")
		return
	}
	err = i.node.FulfillOrder(&fulfill, contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTOrderComplete(w http.ResponseWriter, r *http.Request) {
	checkRatingValue := func(val int) bool {
		if val < core.RatingMin || val > core.RatingMax {
			ErrorResponse(w, http.StatusBadRequest, "rating values must be between 1 and 5")
			return false
		}
		return true
	}
	decoder := json.NewDecoder(r.Body)
	var or core.OrderRatings
	err := decoder.Decode(&or)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, err := i.node.Datastore.Purchases().GetByOrderId(or.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	if state != pb.OrderState_FULFILLED &&
		state != pb.OrderState_RESOLVED &&
		state != pb.OrderState_PAYMENT_FINALIZED {
		errorString := fmt.Sprintf("must be one of the following states to leave a rating and complete the order: %s, %s, %s",
			pb.OrderState_FULFILLED.String(),
			pb.OrderState_RESOLVED.String(),
			pb.OrderState_PAYMENT_FINALIZED.String(),
		)
		ErrorResponse(w, http.StatusBadRequest, errorString)
		return
	}

	if len(contract.VendorOrderFulfillment) == 0 && contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		ErrorResponse(w, http.StatusBadRequest, "moderated orders can only be completed if the vendor has fulfilled the order")
		return
	}

	for _, rd := range or.Ratings {
		if rd.Slug == "" {
			ErrorResponse(w, http.StatusBadRequest, "rating must contain the slug")
			return
		}
		if !checkRatingValue(rd.Overall) {
			return
		}
		if !checkRatingValue(rd.Quality) {
			return
		}
		if !checkRatingValue(rd.Description) {
			return
		}
		if !checkRatingValue(rd.DeliverySpeed) {
			return
		}
		if !checkRatingValue(rd.CustomerService) {
			return
		}
		if len(rd.Review) > core.ReviewMaxCharacters {
			ErrorResponse(w, http.StatusBadRequest, "too many characters in review")
			return
		}
	}

	err = i.node.CompleteOrder(&or, contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTOpenDispute(w http.ResponseWriter, r *http.Request) {
	type dispute struct {
		OrderID string `json:"orderId"`
		Claim   string `json:"claim"`
	}
	decoder := json.NewDecoder(r.Body)
	var d dispute
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var isSale bool
	var contract *pb.RicardianContract
	var state pb.OrderState
	var records []*wallet.TransactionRecord
	contract, state, _, records, _, err = i.node.Datastore.Purchases().GetByOrderId(d.OrderID)
	if err != nil {
		contract, state, _, records, _, err = i.node.Datastore.Sales().GetByOrderId(d.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
		isSale = true
	}
	if contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		ErrorResponse(w, http.StatusBadRequest, "Only moderated orders can be disputed")
		return
	}

	if isSale && (state != pb.OrderState_PARTIALLY_FULFILLED && state != pb.OrderState_FULFILLED) {
		ErrorResponse(w, http.StatusBadRequest, "Order must be either PARTIALLY_FULFILLED or FULFILLED to start a dispute")
		return
	}
	if !isSale && !(state == pb.OrderState_AWAITING_FULFILLMENT || state == pb.OrderState_PENDING || state == pb.OrderState_PARTIALLY_FULFILLED || state == pb.OrderState_FULFILLED || state == pb.OrderState_PROCESSING_ERROR) {
		ErrorResponse(w, http.StatusBadRequest, "Order must be either AWAITING_FULFILLMENT, PARTIALLY_FULFILLED, PENDING, PROCESSING_ERROR or FULFILLED to start a dispute")
		return
	}

	if !isSale && state == pb.OrderState_PROCESSING_ERROR && len(records) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "Cannot dispute an unfunded order")
		return
	}

	err = i.node.OpenDispute(d.OrderID, contract, records, d.Claim)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTCloseDispute(w http.ResponseWriter, r *http.Request) {
	type dispute struct {
		OrderID          string  `json:"orderId"`
		Resolution       string  `json:"resolution"`
		BuyerPercentage  float32 `json:"buyerPercentage"`
		VendorPercentage float32 `json:"vendorPercentage"`
	}
	decoder := json.NewDecoder(r.Body)
	var d dispute
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	err = i.node.CloseDispute(d.OrderID, d.BuyerPercentage, d.VendorPercentage, d.Resolution)
	if err != nil {
		switch err {
		case core.ErrCaseNotFound:
			ErrorResponse(w, http.StatusNotFound, err.Error())
		case core.ErrCloseFailureCaseExpired:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETCase(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)
	buyerContract, vendorContract, buyerErrors, vendorErrors, state, read, date, buyerOpened, claim, resolution, err := i.node.Datastore.Cases().GetCaseMetadata(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	resp := new(pb.CaseRespApi)
	ts, err := ptypes.TimestampProto(date)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.BuyerContract = buyerContract
	resp.VendorContract = vendorContract
	resp.BuyerOpened = buyerOpened
	resp.BuyerContractValidationErrors = buyerErrors
	resp.VendorContractValidationErrors = vendorErrors
	resp.Read = read
	resp.State = state
	resp.Claim = claim
	resp.Resolution = resolution
	resp.Timestamp = ts

	unread, err := i.node.Datastore.Chat().GetUnreadCount(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.UnreadChatMessages = uint64(unread)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, _ := getJSONOutput(m, w, resp)

	i.node.Datastore.Cases().MarkAsRead(orderID)
	SanitizedResponseM(w, out, new(pb.CaseRespApi))
}

func (i *jsonAPIHandler) POSTReleaseFunds(w http.ResponseWriter, r *http.Request) {
	type release struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var rel release
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var contract *pb.RicardianContract
	var state pb.OrderState
	var records []*wallet.TransactionRecord
	contract, state, _, records, _, err = i.node.Datastore.Purchases().GetByOrderId(rel.OrderID)
	if err != nil {
		contract, state, _, records, _, err = i.node.Datastore.Sales().GetByOrderId(rel.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
	}
	if state == pb.OrderState_DECIDED {
		err = i.node.ReleaseFunds(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		ErrorResponse(w, http.StatusBadRequest, "releasefunds can only be called for decided disputes")
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTReleaseEscrow(w http.ResponseWriter, r *http.Request) {
	var (
		rel struct {
			OrderID string `json:"orderId"`
		}
		contract *pb.RicardianContract
		state    pb.OrderState
		records  []*wallet.TransactionRecord
	)

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	contract, state, _, records, _, err = i.node.Datastore.Sales().GetByOrderId(rel.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Order not found")
		return
	}

	if state != pb.OrderState_PENDING && state != pb.OrderState_FULFILLED && state != pb.OrderState_DISPUTED {
		ErrorResponse(w, http.StatusBadRequest, "Release escrow can only be called when sale is pending, fulfilled, or disputed")
		return
	}

	activeDispute, err := i.node.DisputeIsActive(contract)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if activeDispute {
		ErrorResponse(w, http.StatusBadRequest, "Release escrow can only be called after dispute has expired")
		return
	}

	if !(&repo.SaleRecord{Contract: contract}).SupportsTimedEscrowRelease() {
		ErrorResponse(w, http.StatusBadRequest, "Escrowed currency does not support automatic release of funds to vendor")
		return
	}

	err = i.node.ReleaseFundsAfterTimeout(contract, records)
	if err != nil {
		switch err {
		case core.ErrPrematureReleaseOfTimedoutEscrowFunds:
			ErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		case core.EscrowTimeLockedError:
			ErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		default:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	err = i.node.SendFundsReleasedByVendor(contract.BuyerOrder.BuyerID.PeerID, contract.BuyerOrder.BuyerID.Pubkeys.Identity, rel.OrderID)
	if err != nil {
		log.Errorf("SendFundsReleasedByVendor error: %s", err.Error())
		log.Errorf("SendFundsReleasedByVendor: peerID: %s orderID: %s", contract.BuyerOrder.BuyerID.PeerID, rel.OrderID)
	}

	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTRefund(w http.ResponseWriter, r *http.Request) {
	type orderCancel struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var can orderCancel
	err := decoder.Decode(&can)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, err := i.node.Datastore.Sales().GetByOrderId(can.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	if state != pb.OrderState_AWAITING_FULFILLMENT && state != pb.OrderState_PARTIALLY_FULFILLED {
		ErrorResponse(w, http.StatusBadRequest, "order must be AWAITING_FULFILLMENT, or PARTIALLY_FULFILLED")
		return
	}
	err = i.node.RefundOrder(contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	type orderConf struct {
		OrderID string `json:"orderId"`
		Reject  bool   `json:"reject"`
	}
	decoder := json.NewDecoder(r.Body)
	var conf orderConf
	err := decoder.Decode(&conf)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, funded, records, _, err := i.node.Datastore.Sales().GetByOrderId(conf.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	if state != pb.OrderState_PENDING {
		ErrorResponse(w, http.StatusBadRequest, "order has already been confirmed")
		return
	}
	if !funded && !conf.Reject {
		ErrorResponse(w, http.StatusBadRequest, "payment address must be funded before confirmation")
		return
	}
	if !conf.Reject {
		err := i.node.ConfirmOfflineOrder(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		err := i.node.RejectOfflineOrder(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTOrderCancel(w http.ResponseWriter, r *http.Request) {
	type orderCancel struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var can orderCancel
	err := decoder.Decode(&can)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, err := i.node.Datastore.Purchases().GetByOrderId(can.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	if !((state == pb.OrderState_PENDING || state == pb.OrderState_PROCESSING_ERROR) && len(records) > 0) || !(state == pb.OrderState_PENDING || state == pb.OrderState_PROCESSING_ERROR) || contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		ErrorResponse(w, http.StatusBadRequest, "order must be PENDING or PROCESSING_ERROR and only a direct payment to cancel")
		return
	}
	err = i.node.CancelOfflineOrder(contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETOrder(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)
	var err error
	var isSale bool
	var contract *pb.RicardianContract
	var state pb.OrderState
	var funded bool
	var records []*wallet.TransactionRecord
	var read bool
	contract, state, funded, records, read, err = i.node.Datastore.Purchases().GetByOrderId(orderID)
	if err != nil {
		contract, state, funded, records, read, err = i.node.Datastore.Sales().GetByOrderId(orderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
		isSale = true
	}
	resp := new(pb.OrderRespApi)
	resp.Contract = contract
	resp.Funded = funded
	resp.Read = read
	resp.State = state

	paymentTxs, refundTx, err := i.node.BuildTransactionRecords(contract, records, state)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.PaymentAddressTransactions = paymentTxs
	resp.RefundAddressTransaction = refundTx

	unread, err := i.node.Datastore.Chat().GetUnreadCount(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.UnreadChatMessages = uint64(unread)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, _ := getJSONOutput(m, w, resp)
	if isSale {
		i.node.Datastore.Sales().MarkAsRead(orderID)
	} else {
		i.node.Datastore.Purchases().MarkAsRead(orderID)
	}
	SanitizedResponseM(w, out, new(pb.OrderRespApi))
}
