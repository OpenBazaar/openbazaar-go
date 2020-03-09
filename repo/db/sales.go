package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	btc "github.com/btcsuite/btcutil"
)

type SalesDB struct {
	modelStore
}

func NewSaleStore(db *sql.DB, lock *sync.Mutex) repo.SaleStore {
	return &SalesDB{modelStore{db, lock}}
}

func (s *SalesDB) Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if contract.BuyerOrder == nil || contract.BuyerOrder.Payment == nil {
		return errors.New("BuyerOrder and BuyerOrder.Payment must not be nil")
	}

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

	stm := `insert or replace into sales(orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, paymentCoin, coinType, funded, transactions) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,(select funded from sales where orderID="` + orderID + `"),(select transactions from sales where orderID="` + orderID + `"))`
	stmt, err := s.db.Prepare(stm)
	if err != nil {
		return fmt.Errorf("prepare sale sql: %s", err.Error())
	}
	defer stmt.Close()

	handle := contract.BuyerOrder.BuyerID.Handle
	shippingName := ""
	shippingAddress := ""
	if contract.BuyerOrder.Shipping != nil {
		shippingName = contract.BuyerOrder.Shipping.ShipTo
		shippingAddress = contract.BuyerOrder.Shipping.Address
	}
	var address string
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_DIRECT || contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		address = contract.BuyerOrder.Payment.Address
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
		address = contract.VendorOrderConfirmation.PaymentAddress
	}

	paymentCoin, err := PaymentCoinForContract(&contract)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(
		orderID,
		out,
		int(state),
		readInt,
		int(contract.BuyerOrder.Timestamp.Seconds),
		contract.BuyerOrder.Payment.BigAmount,
		contract.VendorListings[0].Item.Images[0].Tiny,
		contract.BuyerOrder.BuyerID.PeerID,
		handle,
		contract.VendorListings[0].Item.Title,
		shippingName,
		shippingAddress,
		address,
		paymentCoin,
		CoinTypeForContract(&contract),
	)
	if err != nil {
		return fmt.Errorf("commit sale: %s", err.Error())
	}

	return nil
}

func (s *SalesDB) MarkAsRead(orderID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.Exec("update sales set read=? where orderID=?", 1, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (s *SalesDB) MarkAsUnread(orderID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.Exec("update sales set read=? where orderID=?", 0, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (s *SalesDB) UpdateFunding(orderId string, funded bool, records []*wallet.TransactionRecord) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	fundedInt := 0
	if funded {
		fundedInt = 1
	}
	serializedTransactions, err := json.Marshal(records)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("update sales set funded=?, transactions=? where orderID=?", fundedInt, string(serializedTransactions), orderId)
	if err != nil {
		return err
	}
	return nil
}

func (s *SalesDB) Delete(orderID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.Exec("delete from sales where orderID=?", orderID)
	if err != nil {
		return err
	}
	return nil
}

func (s *SalesDB) GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]repo.Sale, int, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	q := query{
		table:           "sales",
		columns:         []string{"orderID", "contract", "timestamp", "total", "title", "thumbnail", "buyerID", "buyerHandle", "shippingName", "shippingAddress", "state", "read", "coinType", "paymentCoin"},
		stateFilter:     stateFilter,
		searchTerm:      searchTerm,
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "buyerID", "buyerHandle", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: sortByAscending,
		sortByRead:      sortByRead,
		id:              "orderID",
		exclude:         exclude,
		limit:           limit,
	}
	stm, args := filterQuery(q)
	rows, err := s.db.Query(stm, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var ret []repo.Sale
	for rows.Next() {
		var orderID, title, thumbnail, buyerID, buyerHandle, shippingName, shippingAddr, coinType, paymentCoin string
		var timestamp, stateInt, readInt int
		var contract []byte
		totalStr := ""
		if err := rows.Scan(&orderID, &contract, &timestamp, &totalStr, &title, &thumbnail, &buyerID, &buyerHandle, &shippingName, &shippingAddr, &stateInt, &readInt, &coinType, &paymentCoin); err != nil {
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
		if len(rc.VendorListings) > 0 {
			slug = rc.VendorListings[0].Slug
		}

		var moderated bool
		if rc.BuyerOrder != nil && rc.BuyerOrder.Payment != nil && rc.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
			moderated = true
		}

		if len(rc.VendorListings) > 0 && rc.VendorListings[0].Metadata != nil && rc.VendorListings[0].Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY {
			coinType = ""
		}

		if strings.Contains(totalStr, "e") {
			flt, _, err := big.ParseFloat(totalStr, 10, 0, big.ToNearestEven)
			if err != nil {
				return nil, 0, err
			}
			var i = new(big.Int)
			i, _ = flt.Int(i)
			totalStr = i.String()
		}

		cv, err := repo.NewCurrencyValueWithLookup(totalStr, paymentCoin)
		if err != nil {
			return nil, 0, err
		}

		ret = append(ret, repo.Sale{
			OrderId:         orderID,
			Slug:            slug,
			Timestamp:       time.Unix(int64(timestamp), 0),
			Title:           title,
			Thumbnail:       thumbnail,
			Total:           *cv,
			BuyerId:         buyerID,
			BuyerHandle:     buyerHandle,
			ShippingName:    shippingName,
			ShippingAddress: shippingAddr,
			CoinType:        coinType,
			PaymentCoin:     paymentCoin,
			State:           pb.OrderState(stateInt).String(),
			Read:            read,
			Moderated:       moderated,
		})
	}
	q.columns = []string{"Count(*)"}
	q.limit = -1
	q.exclude = []string{}
	stm, args = filterQuery(q)
	row := s.db.QueryRow(stm, args...)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return ret, 0, err
	}
	return ret, count, nil
}

func (s *SalesDB) GetByPaymentAddress(addr btc.Address) (*pb.RicardianContract, pb.OrderState, bool, []*wallet.TransactionRecord, error) {
	if addr == nil {
		return nil, pb.OrderState(0), false, nil, fmt.Errorf("unable to find sale with nil payment address")
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	stmt, err := s.db.Prepare("select contract, state, funded, transactions from sales where paymentAddr=?")
	if err != nil {
		return nil, pb.OrderState(0), false, nil, err
	}
	defer stmt.Close()

	var (
		contract               []byte
		stateInt               int
		fundedInt              *int
		serializedTransactions []byte
	)
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

func (s *SalesDB) GetByOrderId(orderId string) (*pb.RicardianContract, pb.OrderState, bool, []*wallet.TransactionRecord, bool, *repo.CurrencyCode, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	stmt, err := s.db.Prepare("select contract, state, funded, transactions, read, paymentCoin from sales where orderID=?")
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, nil, err
	}
	defer stmt.Close()
	var (
		contract               []byte
		stateInt               int
		fundedInt              *int
		readInt                *int
		serializedTransactions []byte
		paymentCoin            string
	)
	err = stmt.QueryRow(orderId).Scan(&contract, &stateInt, &fundedInt, &serializedTransactions, &readInt, &paymentCoin)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, nil, err
	}
	rc := new(pb.RicardianContract)
	err = jsonpb.UnmarshalString(string(contract), rc)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, nil, err
	}
	funded := false
	if fundedInt != nil && *fundedInt == 1 {
		funded = true
	}
	read := false
	if readInt != nil && *readInt == 1 {
		read = true
	}
	def, err := repo.AllCurrencies().Lookup(paymentCoin)
	if err != nil {
		return nil, pb.OrderState(0), false, nil, false, nil, fmt.Errorf("validating payment coin: %s", err.Error())
	}
	var records []*wallet.TransactionRecord
	if len(serializedTransactions) > 0 {
		err = json.Unmarshal(serializedTransactions, &records)
		if err != nil {
			return nil, pb.OrderState(0), false, nil, false, nil, fmt.Errorf("unmarshal purchase transactions: %s", err.Error())
		}
	}
	cc := def.CurrencyCode()
	return rc, pb.OrderState(stateInt), funded, records, read, &cc, nil
}

func (s *SalesDB) Count() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	row := s.db.QueryRow("select Count(*) from sales")
	var count int
	err := row.Scan(&count)
	if err != nil {
		log.Error(err)
	}
	return count
}

func (s *SalesDB) GetUnfunded() ([]repo.UnfundedOrder, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	var ret []repo.UnfundedOrder
	rows, err := s.db.Query(`select orderID, contract, timestamp, paymentAddr from sales where state=?`, 1)
	if err != nil {
		return ret, err
	}

	defer rows.Close()
	for rows.Next() {
		var orderID, paymentAddr string
		var timestamp int
		var contractBytes []byte
		err := rows.Scan(&orderID, &contractBytes, &timestamp, &paymentAddr)
		if err != nil {
			return ret, err
		}
		if timestamp > 0 {
			rc := new(pb.RicardianContract)
			err = jsonpb.UnmarshalString(string(contractBytes), rc)
			if err != nil {
				return ret, err
			}
			order, err := repo.ToV5Order(rc.BuyerOrder, repo.AllCurrencies().Lookup)
			if err != nil {
				return ret, err
			}
			ret = append(ret, repo.UnfundedOrder{OrderId: orderID, Timestamp: time.Unix(int64(timestamp), 0), PaymentCoin: order.Payment.AmountCurrency.Code, PaymentAddress: paymentAddr})
		}
	}
	return ret, nil
}

// GetSalesForDisputeTimeoutNotification returns []*SaleRecord including
// each record which needs Notifications to be generated.
func (s *SalesDB) GetSalesForDisputeTimeoutNotification() ([]*repo.SaleRecord, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	stmt := fmt.Sprintf("select orderID, contract, state, timestamp, lastDisputeTimeoutNotifiedAt from sales where (lastDisputeTimeoutNotifiedAt - timestamp) < %d and state in (%d, %d)",
		int(repo.VendorDisputeTimeout_lastInterval.Seconds()),
		pb.OrderState_PARTIALLY_FULFILLED,
		pb.OrderState_FULFILLED,
	)
	rows, err := s.db.Query(stmt)
	if err != nil {
		return nil, fmt.Errorf("selecting sales: %s", err.Error())
	}

	result := make([]*repo.SaleRecord, 0)
	for rows.Next() {
		var (
			lastDisputeTimeoutNotifiedAt int64
			contract                     []byte
			stateInt                     int

			r = &repo.SaleRecord{
				Contract: &pb.RicardianContract{},
			}
			timestamp = sql.NullInt64{}
		)
		if err := rows.Scan(&r.OrderID, &contract, &stateInt, &timestamp, &lastDisputeTimeoutNotifiedAt); err != nil {
			return nil, fmt.Errorf("scanning sales: %s", err.Error())
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

// UpdateSalesLastDisputeTimeoutNotifiedAt accepts []*repo.SaleRecord and updates
// each SaleRecord by their OrderID to the set LastDisputeTimeoutNotifiedAt value. The
// update will be attempted atomically with a rollback attempted in the event of
// an error.
func (s *SalesDB) UpdateSalesLastDisputeTimeoutNotifiedAt(sales []*repo.SaleRecord) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	tx, err := s.BeginTransaction()
	if err != nil {
		return fmt.Errorf("begin update sale transaction: %s", err.Error())
	}
	for _, sale := range sales {
		_, err = tx.Exec("update sales set lastDisputeTimeoutNotifiedAt = ? where orderID = ?", int(sale.LastDisputeTimeoutNotifiedAt.Unix()), sale.OrderID)
		if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				return fmt.Errorf("update sale: (%s) w rollback error: (%s)", err.Error(), rErr.Error())
			}
			return fmt.Errorf("update sale: %s", err.Error())
		}
	}
	if err = tx.Commit(); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return fmt.Errorf("commit sale: (%s) w rollback error: (%s)", err.Error(), rErr.Error())
		}
		return fmt.Errorf("commit sale: %s", err.Error())
	}

	return nil
}
