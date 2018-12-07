package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	wallet "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func (i *jsonAPIHandler) GETAddress(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	if coinType == "address" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			ret[ct.CurrencyCode()] = wal.CurrentAddress(wallet.EXTERNAL).String()
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	addr := wal.CurrentAddress(wallet.EXTERNAL)
	SanitizedResponse(w, fmt.Sprintf(`{"address": "%s"}`, addr.EncodeAddress()))
}

func (i *jsonAPIHandler) GETMnemonic(w http.ResponseWriter, r *http.Request) {
	mn, err := i.node.Datastore.Config().GetMnemonic()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, fmt.Sprintf(`{"mnemonic": "%s"}`, mn))
}

func (i *jsonAPIHandler) GETBalance(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	type balance struct {
		Confirmed   int64  `json:"confirmed"`
		Unconfirmed int64  `json:"unconfirmed"`
		Height      uint32 `json:"height"`
	}
	if coinType == "balance" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			height, _ := wal.ChainTip()
			confirmed, unconfirmed := wal.Balance()
			ret[ct.CurrencyCode()] = balance{Confirmed: confirmed, Unconfirmed: unconfirmed, Height: height}
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	height, _ := wal.ChainTip()
	confirmed, unconfirmed := wal.Balance()
	bal := balance{Confirmed: confirmed, Unconfirmed: unconfirmed, Height: height}
	out, err := json.MarshalIndent(bal, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) POSTSpendCoinsForOrder(w http.ResponseWriter, r *http.Request) {
	var spendArgs core.SpendRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if spendArgs.OrderID == "" {
		ErrorResponse(w, http.StatusBadRequest, core.ErrOrderNotFound.Error())
		return
	}

	spendArgs.RequireAssociatedOrder = true
	result, err := i.node.Spend(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ser, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) POSTSpendCoins(w http.ResponseWriter, r *http.Request) {
	var spendArgs core.SpendRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := i.node.Spend(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ser, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) GETExchangeRate(w http.ResponseWriter, r *http.Request) {
	s := strings.Split(r.URL.Path, "/")
	var currencyCode, coinType string
	if len(s) <= 5 {
		coinType = s[3]
	}
	if len(s) >= 5 {
		currencyCode = s[4]
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if currencyCode == "" || strings.ToLower(currencyCode) == "exchangerate" {
		currencyMap, err := wal.ExchangeRates().GetAllRates(true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		exchangeRateJSON, err := json.MarshalIndent(currencyMap, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(exchangeRateJSON))

	} else {
		rate, err := wal.ExchangeRates().GetExchangeRate(core.NormalizeCurrencyCode(currencyCode))
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		fmt.Fprintf(w, `%.2f`, rate)
	}
}

func (i *jsonAPIHandler) POSTResyncBlockchain(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	creationDate, err := i.node.Datastore.Config().GetCreationDate()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if coinType == "resyncblockchain" {
		for _, wal := range i.node.Multiwallet {
			wal.ReSyncBlockchain(creationDate)
		}
		SanitizedResponse(w, `{}`)
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	wal.ReSyncBlockchain(creationDate)
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETTransactions(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	l := r.URL.Query().Get("limit")
	if l == "" {
		l = "-1"
	}
	limit, err := strconv.Atoi(l)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	offsetID := r.URL.Query().Get("offsetId")
	type Tx struct {
		Txid          string    `json:"txid"`
		Value         int64     `json:"value"`
		Address       string    `json:"address"`
		Status        string    `json:"status"`
		ErrorMessage  string    `json:"errorMessage"`
		Memo          string    `json:"memo"`
		Timestamp     time.Time `json:"timestamp"`
		Confirmations int32     `json:"confirmations"`
		Height        int32     `json:"height"`
		OrderID       string    `json:"orderId"`
		Thumbnail     string    `json:"thumbnail"`
		CanBumpFee    bool      `json:"canBumpFee"`
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	transactions, err := wal.Transactions()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	metadata, err := i.node.Datastore.TxMetadata().GetAll()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	var txs []Tx
	passedOffset := false
	for i := len(transactions) - 1; i >= 0; i-- {
		t := transactions[i]
		tx := Tx{
			Txid:          t.Txid,
			Value:         t.Value,
			Timestamp:     t.Timestamp,
			Confirmations: int32(t.Confirmations),
			Height:        t.Height,
			Status:        string(t.Status),
			CanBumpFee:    true,
			ErrorMessage:  t.ErrorMessage,
		}
		m, ok := metadata[t.Txid]
		if ok {
			tx.Address = m.Address
			tx.Memo = m.Memo
			tx.OrderID = m.OrderId
			tx.Thumbnail = m.Thumbnail
			tx.CanBumpFee = m.CanBumpFee
		}
		if t.Status == wallet.StatusDead {
			tx.CanBumpFee = false
		}
		if offsetID == "" || passedOffset {
			txs = append(txs, tx)
		}
		if t.Txid == offsetID {
			passedOffset = true
		}
		if len(txs) >= limit && limit != -1 {
			break
		}
	}
	type txWithCount struct {
		Transactions []Tx `json:"transactions"`
		Count        int  `json:"count"`
	}
	txns := txWithCount{txs, len(transactions)}
	ret, err := json.MarshalIndent(txns, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTBumpFee(w http.ResponseWriter, r *http.Request) {
	_, txid := path.Split(r.URL.Path)
	txHash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	var wal wallet.Wallet
	for _, w := range i.node.Multiwallet {
		_, err := w.GetTransaction(*txHash)
		if err == nil {
			wal = w
			break
		}
	}
	if wal == nil {
		ErrorResponse(w, http.StatusBadRequest, "transaction not found in any wallet")
		return
	}
	newTxid, err := wal.BumpFee(*txHash)
	if err != nil {
		if err == spvwallet.BumpFeeAlreadyConfirmedError {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
		} else if err == spvwallet.BumpFeeTransactionDeadError {
			ErrorResponse(w, http.StatusMethodNotAllowed, err.Error())
		} else if err == spvwallet.BumpFeeNotFoundError {
			ErrorResponse(w, http.StatusNotFound, err.Error())
		} else {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	m, err := i.node.Datastore.TxMetadata().Get(txid)
	if err != nil {
		m = repo.Metadata{}
	}
	m.Txid = txid
	m.CanBumpFee = false
	if err := i.node.Datastore.TxMetadata().Put(m); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := i.node.Datastore.TxMetadata().Put(repo.Metadata{
		Txid:       newTxid.String(),
		Address:    "",
		Memo:       fmt.Sprintf("Fee bump of %s", txid),
		OrderId:    "",
		Thumbnail:  "",
		CanBumpFee: true,
	}); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	type response struct {
		Txid               string    `json:"txid"`
		Amount             int64     `json:"amount"`
		ConfirmedBalance   int64     `json:"confirmedBalance"`
		UnconfirmedBalance int64     `json:"unconfirmedBalance"`
		Timestamp          time.Time `json:"timestamp"`
		Memo               string    `json:"memo"`
	}
	confirmed, unconfirmed := wal.Balance()
	txn, err := wal.GetTransaction(*newTxid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := &response{
		Txid:               newTxid.String(),
		ConfirmedBalance:   confirmed,
		UnconfirmedBalance: unconfirmed,
		Amount:             -(txn.Value),
		Timestamp:          txn.Timestamp,
		Memo:               fmt.Sprintf("Fee bump of %s", txid),
	}
	ser, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) GETEstimateFee(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)

	fl := r.URL.Query().Get("feeLevel")
	amt := r.URL.Query().Get("amount")
	amount, err := strconv.Atoi(amt)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var feeLevel wallet.FeeLevel
	switch strings.ToUpper(fl) {
	case "PRIORITY":
		feeLevel = wallet.PRIOIRTY
	case "NORMAL":
		feeLevel = wallet.NORMAL
	case "ECONOMIC":
		feeLevel = wallet.ECONOMIC
	default:
		ErrorResponse(w, http.StatusBadRequest, "Unknown feeLevel")
		return
	}

	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}

	fee, err := wal.EstimateSpendFee(int64(amount), feeLevel)
	if err != nil {
		switch {
		case err == wallet.ErrorInsuffientFunds:
			ErrorResponse(w, http.StatusBadRequest, `ERROR_INSUFFICIENT_FUNDS`)
			return
		case err == wallet.ErrorDustAmount:
			ErrorResponse(w, http.StatusBadRequest, `ERROR_DUST_AMOUNT`)
			return
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	fmt.Fprintf(w, `{"estimatedFee": %d}`, (fee))
}

func (i *jsonAPIHandler) GETFees(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	type fees struct {
		Priority uint64 `json:"priority"`
		Normal   uint64 `json:"normal"`
		Economic uint64 `json:"economic"`
	}
	if coinType == "fees" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			priority := wal.GetFeePerByte(wallet.PRIOIRTY)
			normal := wal.GetFeePerByte(wallet.NORMAL)
			economic := wal.GetFeePerByte(wallet.ECONOMIC)
			ret[ct.CurrencyCode()] = fees{Priority: priority, Normal: normal, Economic: economic}
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	priority := wal.GetFeePerByte(wallet.PRIOIRTY)
	normal := wal.GetFeePerByte(wallet.NORMAL)
	economic := wal.GetFeePerByte(wallet.ECONOMIC)
	f := fees{Priority: priority, Normal: normal, Economic: economic}
	out, err := json.MarshalIndent(f, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) GETWalletStatus(w http.ResponseWriter, r *http.Request) {

	_, coinType := path.Split(r.URL.Path)
	type status struct {
		Height   uint32 `json:"height"`
		BestHash string `json:"bestHash"`
	}
	if coinType == "status" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			height, hash := wal.ChainTip()
			ret[ct.CurrencyCode()] = status{height, hash.String()}
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	height, hash := wal.ChainTip()
	st := status{height, hash.String()}
	out, err := json.MarshalIndent(st, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) POSTEstimateTotal(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	amount, err := i.node.EstimateOrderTotal(&data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprintf(w, "%d", int(amount))
}
