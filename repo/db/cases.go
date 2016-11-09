package db

import (
	"database/sql"
	"errors"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"strings"
	"sync"
)

type CasesDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (c *CasesDB) Put(orderID string, buyerContract, vendorContract pb.RicardianContract, state pb.OrderState, read bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

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
	buyerOut, err := m.MarshalToString(&buyerContract)
	if err != nil {
		return err
	}
	vendorOut, err := m.MarshalToString(&vendorContract)
	if err != nil {
		return err
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into cases(orderID, buyerContract, vendorContract, state, read, date, thumbnail, buyerID, buyerBlockchainID, vendorID, vendorBlockchainID, title) values(?,?,?,?,?,?,?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}
	var contract pb.RicardianContract
	if &buyerContract != nil {
		contract = buyerContract
	} else if &vendorContract != nil {
		contract = vendorContract
	} else {
		return errors.New("Both contracts cannot be nil")
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		orderID,
		buyerOut,
		vendorOut,
		int(state),
		readInt,
		int(contract.BuyerOrder.Timestamp.Seconds),
		contract.VendorListings[0].Item.Images[0].Tiny,
		contract.BuyerOrder.BuyerID.Guid,
		contract.BuyerOrder.BuyerID.BlockchainID,
		contract.VendorListings[0].VendorID.Guid,
		contract.VendorListings[0].VendorID.BlockchainID,
		strings.ToLower(contract.VendorListings[0].Item.Title),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *CasesDB) MarkAsRead(orderID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("update cases set read=? where orderID=?", 1, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) Delete(orderID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("delete from cases where orderID=?", orderID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) GetAll() ([]string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stm := "select orderID from cases"
	rows, err := c.db.Query(stm)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	var ret []string
	for rows.Next() {
		var orderID string
		if err := rows.Scan(&orderID); err != nil {
			return ret, err
		}
		ret = append(ret, orderID)
	}
	return ret, nil
}

func (c *CasesDB) GetByOrderId(orderId string) (buyerContract, vendorContract *pb.RicardianContract, state pb.OrderState, read bool, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stmt, err := c.db.Prepare("select buyerContract, vendorContract, state, read from cases where orderID=?")
	defer stmt.Close()
	var buyerCon []byte
	var vendorCon []byte
	var stateInt int
	var readInt *int
	err = stmt.QueryRow(orderId).Scan(&buyerCon, &vendorCon, &stateInt, &readInt)
	if err != nil {
		return nil, nil, pb.OrderState(0), false, err
	}
	brc := new(pb.RicardianContract)
	err = jsonpb.UnmarshalString(string(buyerCon), brc)
	if err != nil {
		return nil, nil, pb.OrderState(0), false, err
	}
	vrc := new(pb.RicardianContract)
	err = jsonpb.UnmarshalString(string(vendorCon), vrc)
	if err != nil {
		return nil, nil, pb.OrderState(0), false, err
	}
	read = false
	if readInt != nil && *readInt == 1 {
		read = true
	}
	return brc, vrc, pb.OrderState(stateInt), read, nil
}
