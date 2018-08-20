package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	btc "github.com/btcsuite/btcutil"
)

type PurchasesDB struct {
	modelStore
}

func NewPurchaseStore(db *sql.DB, lock *sync.Mutex) repo.PurchaseStore {
	return &PurchasesDB{modelStore{db, lock}}
}

func (p *PurchasesDB) Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	readInt := 0
	if read {
		readInt = 1
	}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(&contract)
	if err != nil {
		return err
	}

	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into purchases(orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, paymentCoin, coinType, funded, transactions) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,(select funded from purchases where orderID="` + orderID + `"),(select transactions from purchases where orderID="` + orderID + `"))`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}
	handle := contract.VendorListings[0].VendorID.Handle
	shippingName := ""
	shippingAddress := ""
	if contract.BuyerOrder.Shipping != nil {
		shippingName = contract.BuyerOrder.Shipping.ShipTo
		shippingAddress = contract.BuyerOrder.Shipping.Address
	}
	var paymentAddr string
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_DIRECT || contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		paymentAddr = contract.BuyerOrder.Payment.Address
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
		paymentAddr = contract.VendorOrderConfirmation.PaymentAddress
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		orderID,
		out,
		int(state),
		readInt,
		int(contract.BuyerOrder.Timestamp.Seconds),
		int(contract.BuyerOrder.Payment.Amount),
		contract.VendorListings[0].Item.Images[0].Tiny,
		contract.VendorListings[0].VendorID.PeerID,
		handle,
		contract.VendorListings[0].Item.Title,
		shippingName,
		shippingAddress,
		paymentAddr,
		PaymentCoinForContract(&contract),
		CoinTypeForContract(&contract),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (p *PurchasesDB) MarkAsRead(orderID string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, err := p.db.Exec("update purchases set read=? where orderID=?", 1, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (p *PurchasesDB) MarkAsUnread(orderID string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, err := p.db.Exec("update purchases set read=? where orderID=?", 0, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (p *PurchasesDB) UpdateFunding(orderId string, funded bool, records []*wallet.TransactionRecord) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	fundedInt := 0
	if funded {
		fundedInt = 1
	}
	serializedTransactions, err := json.Marshal(records)
	if err != nil {
		return err
	}
	_, err = p.db.Exec("update purchases set funded=?, transactions=? where orderID=?", fundedInt, string(serializedTransactions), orderId)
	if err != nil {
		return err
	}
	return nil
}

func (p *PurchasesDB) Delete(orderID string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, err := p.db.Exec("delete from purchases where orderID=?", orderID)
	if err != nil {
		return err
	}
	return nil
}

func (p *PurchasesDB) GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]repo.Purchase, int, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	q := query{
		table:           "purchases",
		columns:         []string{"orderID", "contract", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorHandle", "shippingName", "shippingAddress", "state", "read", "coinType", "paymentCoin"},
		stateFilter:     stateFilter,
		searchTerm:      searchTerm,
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorHandle", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: sortByAscending,
		sortByRead:      sortByRead,
		id:              "orderID",
		exclude:         exclude,
		limit:           limit,
	}
	stm, args := filterQuery(q)
	rows, err := p.db.Query(stm, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var ret []repo.Purchase
	for rows.Next() {
		var orderID, title, thumbnail, vendorID, vendorHandle, shippingName, shippingAddr, coinType, paymentCoin string
		var contract []byte
		var timestamp, total, stateInt, readInt int
		if err := rows.Scan(&orderID, &contract, &timestamp, &total, &title, &thumbnail, &vendorID, &vendorHandle, &shippingName, &shippingAddr, &stateInt, &readInt, &coinType, &paymentCoin); err != nil {
			return ret, 0, err
		}
		read := false
		if readInt > 0 {
			read = true
		}

		rc := new(pb.RicardianContract)
		if err := jsonpb.UnmarshalString(string(contract), rc); err != nil {
			return ret, 0, err
		}
		var slug string
		var moderated bool
		if len(rc.VendorListings) > 0 {
			slug = rc.VendorListings[0].Slug
		}
		if rc.BuyerOrder != nil && rc.BuyerOrder.Payment != nil && rc.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
			moderated = true
		}

		ret = append(ret, repo.Purchase{
			OrderId:         orderID,
			Slug:            slug,
			Timestamp:       time.Unix(int64(timestamp), 0),
			Title:           title,
			Thumbnail:       thumbnail,
			Total:           uint64(total),
			VendorId:        vendorID,
			VendorHandle:    vendorHandle,
			ShippingName:    shippingName,
			ShippingAddress: shippingAddr,
			CoinType:        coinType,
			PaymentCoin:     paymentCoin,
			State:           pb.OrderState(stateInt).String(),
			Moderated:       moderated,
			Read:            read,
		})
	}
	q.columns = []string{"Count(*)"}
	q.limit = -1
	q.exclude = []string{}
	stm, args = filterQuery(q)
	row := p.db.QueryRow(stm, args...)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return ret, 0, err
	}
	return ret, count, nil
}

func (p *PurchasesDB) GetByPaymentAddress(addr btc.Address) (*pb.RicardianContract, pb.OrderState, bool, []*wallet.TransactionRecord, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	stmt, err := p.db.Prepare("select contract, state, funded, transactions from purchases where paymentAddr=?")
	if err != nil {
		return nil, pb.OrderState(0), false, nil, err
	}
	defer stmt.Close()
	var contract []byte
	var stateInt int
	var fundedInt *int
	var serializedTransactions []byte
	err = stmt.QueryRow(addr.EncodeAddress()).Scan(&contract, &stateInt, &fundedInt, &serializedTransactions)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, err
	}
	rc := new(pb.RicardianContract)
	err = jsonpb.UnmarshalString(string(contract), rc)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, err
	}
	funded := false
	if fundedInt != nil && *fundedInt == 1 {
		funded = true
	}
	var records []*wallet.TransactionRecord
	if len(serializedTransactions) > 0 {
		err = json.Unmarshal(serializedTransactions, &records)
		if err != nil {
			return nil, pb.OrderState(0), false, nil, err
		}
	}
	return rc, pb.OrderState(stateInt), funded, records, nil
}

func (p *PurchasesDB) GetByOrderId(orderId string) (*pb.RicardianContract, pb.OrderState, bool, []*wallet.TransactionRecord, bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	stmt, err := p.db.Prepare("select contract, state, funded, transactions, read from purchases where orderID=?")
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, err
	}
	defer stmt.Close()
	var contract []byte
	var stateInt int
	var fundedInt *int
	var readInt *int
	var serializedTransactions []byte
	err = stmt.QueryRow(orderId).Scan(&contract, &stateInt, &fundedInt, &serializedTransactions, &readInt)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, err
	}
	rc := new(pb.RicardianContract)
	err = jsonpb.UnmarshalString(string(contract), rc)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, err
	}
	funded := false
	if fundedInt != nil && *fundedInt == 1 {
		funded = true
	}
	read := false
	if readInt != nil && *readInt == 1 {
		read = true
	}
	var records []*wallet.TransactionRecord
	json.Unmarshal(serializedTransactions, &records)
	return rc, pb.OrderState(stateInt), funded, records, read, nil
}

func (p *PurchasesDB) Count() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	row := p.db.QueryRow("select Count(*) from purchases")
	var count int
	row.Scan(&count)
	return count
}

func (p *PurchasesDB) GetPurchasesForDisputeExpiryNotification() ([]*repo.PurchaseRecord, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	s := fmt.Sprintf("select orderID, contract, state, timestamp, lastDisputeExpiryNotifiedAt, disputedAt from purchases where (lastDisputeExpiryNotifiedAt - disputedAt) < %d and state = %d",
		int(repo.BuyerDisputeExpiry_lastInterval.Seconds()),
		pb.OrderState_DISPUTED,
	)
	rows, err := p.db.Query(s)
	if err != nil {
		return nil, fmt.Errorf("selecting purchases: %s", err.Error())
	}

	result := make([]*repo.PurchaseRecord, 0)
	for rows.Next() {
		var (
			disputedAt                  int64
			lastDisputeExpiryNotifiedAt int64
			contract                    []byte
			stateInt                    int

			r = &repo.PurchaseRecord{
				Contract: &pb.RicardianContract{},
			}
			timestamp = sql.NullInt64{}
		)
		if err := rows.Scan(&r.OrderID, &contract, &stateInt, &timestamp, &lastDisputeExpiryNotifiedAt, &disputedAt); err != nil {
			return nil, fmt.Errorf("scanning purchases: %s\n", err.Error())
		}
		if err := jsonpb.UnmarshalString(string(contract), r.Contract); err != nil {
			return nil, fmt.Errorf("unmarshaling contract: %s\n", err.Error())
		}
		r.OrderState = pb.OrderState(stateInt)
		if timestamp.Valid {
			r.Timestamp = time.Unix(timestamp.Int64, 0)
		} else {
			r.Timestamp = time.Now()
		}
		r.LastDisputeExpiryNotifiedAt = time.Unix(lastDisputeExpiryNotifiedAt, 0)
		r.DisputedAt = time.Unix(disputedAt, 0)

		result = append(result, r)
	}
	return result, nil
}

// GetPurchasesForDisputeTimeoutNotification returns []*PurchaseRecord including
// each record which needs Notifications to be generated.
func (p *PurchasesDB) GetPurchasesForDisputeTimeoutNotification() ([]*repo.PurchaseRecord, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	s := fmt.Sprintf("select orderID, contract, state, timestamp, lastDisputeTimeoutNotifiedAt from purchases where (lastDisputeTimeoutNotifiedAt - timestamp) < %d and state in (%d, %d, %d)",
		int(repo.BuyerDisputeTimeout_totalDuration.Seconds()),
		pb.OrderState_PENDING,
		pb.OrderState_AWAITING_FULFILLMENT,
		pb.OrderState_FULFILLED,
	)
	rows, err := p.db.Query(s)
	if err != nil {
		return nil, fmt.Errorf("selecting purchases: %s", err.Error())
	}

	result := make([]*repo.PurchaseRecord, 0)
	for rows.Next() {
		var (
			lastDisputeTimeoutNotifiedAt int64
			contract                     []byte
			stateInt                     int

			r = &repo.PurchaseRecord{
				Contract: &pb.RicardianContract{},
			}
			timestamp = sql.NullInt64{}
		)
		if err := rows.Scan(&r.OrderID, &contract, &stateInt, &timestamp, &lastDisputeTimeoutNotifiedAt); err != nil {
			return nil, fmt.Errorf("scanning purchases: %s\n", err.Error())
		}
		if err := jsonpb.UnmarshalString(string(contract), r.Contract); err != nil {
			return nil, fmt.Errorf("unmarshaling contract: %s\n", err.Error())
		}
		r.OrderState = pb.OrderState(stateInt)
		if timestamp.Valid {
			r.Timestamp = time.Unix(timestamp.Int64, 0)
		} else {
			r.Timestamp = time.Now()
		}
		r.LastDisputeTimeoutNotifiedAt = time.Unix(lastDisputeTimeoutNotifiedAt, 0)

		if r.IsDisputeable() {
			result = append(result, r)
		}
	}
	return result, nil
}

// UpdatePurchasesLastDisputeTimeoutNotifiedAt accepts []*repo.PurchaseRecord and updates
// each PurchaseRecord by their OrderID to the set LastDisputeTimeoutNotifiedAt value. The
// update will be attempted atomically with a rollback attempted in the event of
// an error.
func (p *PurchasesDB) UpdatePurchasesLastDisputeTimeoutNotifiedAt(purchases []*repo.PurchaseRecord) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("begin update purchase transaction: %s", err.Error())
	}
	for _, p := range purchases {
		_, err = tx.Exec("update purchases set lastDisputeTimeoutNotifiedAt = ? where orderID = ?", int(p.LastDisputeTimeoutNotifiedAt.Unix()), p.OrderID)
		if err != nil {
			return fmt.Errorf("update purchase: %s", err.Error())
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit update purchase transaction: %s", err.Error())
	}

	return nil
}

// UpdatePurchasesLastDisputeExpiryNotifiedAt accepts []*repo.PurchaseRecord and updates
// each PurchaseRecord by their OrderID to the set LastDisputeExpiryNotifiedAt value. The
// update will be attempted atomically with a rollback attempted in the event of
// an error.
func (p *PurchasesDB) UpdatePurchasesLastDisputeExpiryNotifiedAt(purchases []*repo.PurchaseRecord) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("begin update purchase transaction: %s", err.Error())
	}
	for _, p := range purchases {
		_, err = tx.Exec("update purchases set lastDisputeExpiryNotifiedAt = ? where orderID = ?", int(p.LastDisputeExpiryNotifiedAt.Unix()), p.OrderID)
		if err != nil {
			return fmt.Errorf("update purchase: %s", err.Error())
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit update purchase transaction: %s", err.Error())
	}

	return nil
}
