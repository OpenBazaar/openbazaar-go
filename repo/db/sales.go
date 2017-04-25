package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	btc "github.com/btcsuite/btcutil"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SalesDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (s *SalesDB) Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

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

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into sales(orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerBlockchainID, title, shippingName, shippingAddress, paymentAddr, funded, transactions) values(?,?,?,?,?,?,?,?,?,?,?,?,?,(select funded from sales where orderID="` + orderID + `"),(select transactions from sales where orderID="` + orderID + `"))`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}

	blockchainID := contract.BuyerOrder.BuyerID.BlockchainID
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
	defer stmt.Close()
	_, err = stmt.Exec(
		orderID,
		out,
		int(state),
		readInt,
		int(contract.BuyerOrder.Timestamp.Seconds),
		int(contract.BuyerOrder.Payment.Amount),
		contract.VendorListings[0].Item.Images[0].Tiny,
		contract.BuyerOrder.BuyerID.PeerID,
		blockchainID,
		contract.VendorListings[0].Item.Title,
		shippingName,
		shippingAddress,
		address,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
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

func (s *SalesDB) UpdateFunding(orderId string, funded bool, records []*spvwallet.TransactionRecord) error {
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

func (s *SalesDB) GetAll(offsetId string, limit int, stateFilter []pb.OrderState, searchTerm string) ([]repo.Sale, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	stateFilterClause := ""
	var states []int
	if len(stateFilter) > 0 {
		stateFilterClauseParts := make([]string, 0, len(stateFilter))

		for i := 0; i < len(stateFilter); i++ {
			states = append(states, int(stateFilter[i]))
			stateFilterClauseParts = append(stateFilterClauseParts, "?")
		}

		stateFilterClause = "state in (" + strings.Join(stateFilterClauseParts, ",") + ")"
	}

	var i []interface{}
	var stm string
	var search string
	tables := `('orderID' || 'timestamp' || 'total' || 'title' || 'thumbnail' || 'buyerID' || 'buyerBlockchainID' || 'shippingName' || 'shippingAddress')`
	if offsetId != "" {
		i = append(i, offsetId)
		var filter string
		if stateFilterClause != "" {
			filter = " and " + stateFilterClause
		}
		if searchTerm != "" {
			search = " and " + tables + " like '%?%'"
		}
		stm = "select orderID, timestamp, total, title, thumbnail, buyerID, buyerBlockchainID, shippingName, shippingAddress, state, read from sales where rowid>(select rowid from sales where orderID=?)" + filter + search + " limit " + strconv.Itoa(limit) + " ;"
	} else {
		var filter string
		if stateFilterClause != "" {
			filter = " where " + stateFilterClause
		}
		if searchTerm != "" {
			if filter == "" {
				search = " where " + tables + " like '%?%'"
			} else {
				search = " and " + tables + " like '%?%'"
			}
		}
		stm = "select orderID, timestamp, total, title, thumbnail, buyerID, buyerBlockchainID, shippingName, shippingAddress, state, read from sales" + filter + search + " limit " + strconv.Itoa(limit) + ";"
	}
	fmt.Println(stm)
	for _, s := range states {
		i = append(i, s)
	}
	i = append(i, searchTerm)
	rows, err := s.db.Query(stm, i...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ret []repo.Sale
	for rows.Next() {
		var orderID, title, thumbnail, buyerID, buyerHandle, shippingName, shippingAddr string
		var timestamp, total, stateInt, readInt int
		if err := rows.Scan(&orderID, &timestamp, &total, &title, &thumbnail, &buyerID, &buyerHandle, &shippingName, &shippingAddr, &stateInt, &readInt); err != nil {
			return ret, err
		}
		read := false
		if readInt > 0 {
			read = true
		}

		ret = append(ret, repo.Sale{
			OrderId:         orderID,
			Timestamp:       time.Unix(int64(timestamp), 0),
			Title:           title,
			Thumbnail:       thumbnail,
			Total:           uint64(total),
			BuyerId:         buyerID,
			BuyerHandle:     buyerHandle,
			ShippingName:    shippingName,
			ShippingAddress: shippingAddr,
			State:           pb.OrderState(stateInt).String(),
			Read:            read,
		})
	}
	return ret, nil
}

func (s *SalesDB) GetByPaymentAddress(addr btc.Address) (*pb.RicardianContract, pb.OrderState, bool, []*spvwallet.TransactionRecord, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	stmt, err := s.db.Prepare("select contract, state, funded, transactions from sales where paymentAddr=?")
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
	var records []*spvwallet.TransactionRecord
	json.Unmarshal(serializedTransactions, &records)
	return rc, pb.OrderState(stateInt), funded, records, nil
}

func (s *SalesDB) GetByOrderId(orderId string) (*pb.RicardianContract, pb.OrderState, bool, []*spvwallet.TransactionRecord, bool, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	stmt, err := s.db.Prepare("select contract, state, funded, transactions, read from sales where orderID=?")
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
	var records []*spvwallet.TransactionRecord
	json.Unmarshal(serializedTransactions, &records)
	return rc, pb.OrderState(stateInt), funded, records, read, nil
}
