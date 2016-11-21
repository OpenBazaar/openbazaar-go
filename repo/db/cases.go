package db

import (
	"database/sql"
	"encoding/json"
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

func (c *CasesDB) Put(orderID string, buyerContract, vendorContract *pb.RicardianContract, buyerValidationErrors, vendorValidationErrors []string, state pb.OrderState, read bool, buyerOpened bool, claim string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	readInt := 0
	if read {
		readInt = 1
	}
	buyerOpenedInt := 0
	if buyerOpened {
		buyerOpenedInt = 1
	}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	var buyerOut string
	var vendorOut string
	var err error
	if buyerContract != nil {
		buyerOut, err = m.MarshalToString(buyerContract)
		if err != nil {
			return err
		}
	}
	if vendorContract != nil {
		vendorOut, err = m.MarshalToString(vendorContract)
		if err != nil {
			return err
		}
	}
	buyerErrorsOut, err := json.Marshal(buyerValidationErrors)
	if err != nil {
		return err
	}
	vendorErrorsOut, err := json.Marshal(vendorValidationErrors)
	if err != nil {
		return err
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into cases(orderID, buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, state, read, date, thumbnail, buyerID, buyerBlockchainID, vendorID, vendorBlockchainID, title, buyerOpened, claim) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}
	var contract *pb.RicardianContract
	if buyerContract != nil {
		contract = buyerContract
	} else if vendorContract != nil {
		contract = vendorContract
	} else {
		return errors.New("Both contracts cannot be nil")
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		orderID,
		buyerOut,
		vendorOut,
		string(buyerErrorsOut),
		string(vendorErrorsOut),
		int(state),
		readInt,
		int(contract.BuyerOrder.Timestamp.Seconds),
		contract.VendorListings[0].Item.Images[0].Tiny,
		contract.BuyerOrder.BuyerID.Guid,
		contract.BuyerOrder.BuyerID.BlockchainID,
		contract.VendorListings[0].VendorID.Guid,
		contract.VendorListings[0].VendorID.BlockchainID,
		strings.ToLower(contract.VendorListings[0].Item.Title),
		buyerOpenedInt,
		claim,
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

func (c *CasesDB) GetByOrderId(orderId string) (buyerContract, vendorContract *pb.RicardianContract, buyerValidationErrors, vendorValidationErrors []string, state pb.OrderState, read bool, buyerOpened bool, claim string, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stmt, err := c.db.Prepare("select buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, state, read, buyerOpened, claim from cases where orderID=?")
	defer stmt.Close()
	var buyerCon []byte
	var vendorCon []byte
	var buyerErrors []byte
	var vendorErrors []byte
	var stateInt int
	var readInt *int
	var buyerOpenedInt int
	err = stmt.QueryRow(orderId).Scan(&buyerCon, &vendorCon, &buyerErrors, &vendorErrors, &stateInt, &readInt, &buyerOpenedInt, &claim)
	if err != nil {
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, false, "", err
	}
	brc := new(pb.RicardianContract)
	if string(buyerCon) != "" {
		err = jsonpb.UnmarshalString(string(buyerCon), brc)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, false, "", err
		}
	} else {
		brc = nil
	}
	vrc := new(pb.RicardianContract)
	if string(vendorCon) != "" {
		err = jsonpb.UnmarshalString(string(vendorCon), vrc)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, false, "", err
		}
	} else {
		vrc = nil
	}
	read = false
	if readInt != nil && *readInt == 1 {
		read = true
	}

	if buyerOpenedInt == 1 {
		buyerOpened = true
	}

	var berr []string
	err = json.Unmarshal(buyerErrors, &berr)
	if err != nil {
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, false, "", err
	}
	var verr []string
	err = json.Unmarshal(vendorErrors, &verr)
	if err != nil {
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, false, "", err
	}
	return brc, vrc, berr, verr, pb.OrderState(stateInt), read, buyerOpened, claim, nil
}
