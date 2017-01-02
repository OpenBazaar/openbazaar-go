package db

import (
	"database/sql"
	"encoding/json"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"sync"
	"time"
)

type CasesDB struct {
	db   *sql.DB
	lock *sync.RWMutex
}

func (c *CasesDB) Put(caseID string, state pb.OrderState, buyerOpened bool, claim string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	readInt := 0

	buyerOpenedInt := 0
	if buyerOpened {
		buyerOpenedInt = 1
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into cases(caseID, state, read, date, buyerOpened, claim, buyerPayoutAddress, vendorPayoutAddress) values(?,?,?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		caseID,
		int(state),
		readInt,
		int(time.Now().Unix()),
		buyerOpenedInt,
		claim,
		"",
		"",
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *CasesDB) UpdateBuyerInfo(caseID string, buyerContract *pb.RicardianContract, buyerValidationErrors []string, buyerPayoutAddress string, buyerOutpoints []*pb.Outpoint) error {
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	var buyerOut string
	var err error
	if buyerContract != nil {
		buyerOut, err = m.MarshalToString(buyerContract)
		if err != nil {
			return err
		}
	}
	buyerErrorsOut, err := json.Marshal(buyerValidationErrors)
	if err != nil {
		return err
	}
	var buyerOutpointsOut []byte
	if buyerOutpoints != nil {
		buyerOutpointsOut, err = json.Marshal(buyerOutpoints)
		if err != nil {
			return err
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	_, err = c.db.Exec("update cases set buyerContract=?, buyerValidationErrors=?, buyerPayoutAddress=?, buyerOutpoints=? where caseID=?", buyerOut, string(buyerErrorsOut), buyerPayoutAddress, string(buyerOutpointsOut), caseID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) UpdateVendorInfo(caseID string, vendorContract *pb.RicardianContract, vendorValidationErrors []string, vendorPayoutAddress string, vendorOutpoints []*pb.Outpoint) error {
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	var vendorOut string
	var err error
	if vendorContract != nil {
		vendorOut, err = m.MarshalToString(vendorContract)
		if err != nil {
			return err
		}
	}
	vendorErrorsOut, err := json.Marshal(vendorValidationErrors)
	if err != nil {
		return err
	}
	var vendorOutpointsOut []byte
	if vendorOutpoints != nil {
		vendorOutpointsOut, err = json.Marshal(vendorOutpoints)
		if err != nil {
			return err
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	_, err = c.db.Exec("update cases set vendorContract=?, vendorValidationErrors=?, vendorPayoutAddress=?, vendorOutpoints=? where caseID=?", vendorOut, string(vendorErrorsOut), vendorPayoutAddress, string(vendorOutpointsOut), caseID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) MarkAsRead(orderID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("update cases set read=? where caseID=?", 1, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) MarkAsClosed(caseID string, resolution *pb.DisputeResolution) error {
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	var rOut string
	var err error
	if resolution != nil {
		rOut, err = m.MarshalToString(resolution)
		if err != nil {
			return err
		}
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err = c.db.Exec("update cases set disputeResolution=?, state=? where caseID=?", rOut, int(pb.OrderState_RESOLVED), caseID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) Delete(orderID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("delete from cases where caseID=?", orderID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) GetAll() ([]string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	stm := "select caseID from cases"
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

func (c *CasesDB) GetCaseMetadata(caseID string) (buyerContract, vendorContract *pb.RicardianContract, buyerValidationErrors, vendorValidationErrors []string, state pb.OrderState, read bool, timestamp time.Time, buyerOpened bool, claim string, resolution *pb.DisputeResolution, err error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	stmt, err := c.db.Prepare("select buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, state, read, date, buyerOpened, claim, disputeResolution from cases where caseID=?")
	defer stmt.Close()
	var buyerCon []byte
	var vendorCon []byte
	var buyerErrors []byte
	var vendorErrors []byte
	var stateInt int
	var readInt *int
	var date int
	var buyerOpenedInt int
	var disputResolution []byte
	err = stmt.QueryRow(caseID).Scan(&buyerCon, &vendorCon, &buyerErrors, &vendorErrors, &stateInt, &readInt, &date, &buyerOpenedInt, &claim, &disputResolution)
	if err != nil {
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
	}
	brc := new(pb.RicardianContract)
	if string(buyerCon) != "" {
		err = jsonpb.UnmarshalString(string(buyerCon), brc)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
		}
	} else {
		brc = nil
	}
	vrc := new(pb.RicardianContract)
	if string(vendorCon) != "" {
		err = jsonpb.UnmarshalString(string(vendorCon), vrc)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
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
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
	}
	var verr []string
	err = json.Unmarshal(vendorErrors, &verr)
	if err != nil {
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
	}
	resolution = new(pb.DisputeResolution)
	if string(disputResolution) != "" {
		err = jsonpb.UnmarshalString(string(disputResolution), resolution)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
		}
	} else {
		resolution = nil
	}

	return brc, vrc, berr, verr, pb.OrderState(stateInt), read, time.Unix(int64(date), 0), buyerOpened, claim, resolution, nil
}

func (c *CasesDB) GetPayoutDetails(caseID string) (buyerContract, vendorContract *pb.RicardianContract, buyerPayoutAddress, vendorPayoutAddress string, buyerOutpoints, vendorOutpoints []*pb.Outpoint, state pb.OrderState, err error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	stmt, err := c.db.Prepare("select buyerContract, vendorContract, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state from cases where caseID=?")
	var buyerCon []byte
	var vendorCon []byte
	var buyerOuts []byte
	var vendorOuts []byte
	var buyerAddr string
	var vendorAddr string
	var stateInt int

	err = stmt.QueryRow(caseID).Scan(&buyerCon, &vendorCon, &buyerAddr, &vendorAddr, &buyerOuts, &vendorOuts, &stateInt)
	if err != nil {
		return nil, nil, "", "", nil, nil, pb.OrderState(0), err
	}

	brc := new(pb.RicardianContract)
	if string(buyerCon) != "" {
		err = jsonpb.UnmarshalString(string(buyerCon), brc)
		if err != nil {
			return nil, nil, "", "", nil, nil, pb.OrderState(0), err
		}
	} else {
		brc = nil
	}
	vrc := new(pb.RicardianContract)
	if string(vendorCon) != "" {
		err = jsonpb.UnmarshalString(string(vendorCon), vrc)
		if err != nil {
			return nil, nil, "", "", nil, nil, pb.OrderState(0), err
		}
	} else {
		vrc = nil
	}

	var buyerOutpointsOut []pb.Outpoint
	if len(buyerOuts) > 0 {
		err = json.Unmarshal(buyerOuts, &buyerOutpointsOut)
		if err != nil {
			return nil, nil, "", "", nil, nil, pb.OrderState(0), err
		}
	}
	var vendorOutpointsOut []pb.Outpoint
	if len(vendorOuts) > 0 {
		err = json.Unmarshal(vendorOuts, &vendorOutpointsOut)
		if err != nil {
			return nil, nil, "", "", nil, nil, pb.OrderState(0), err
		}
	}

	toPointer := func(op []pb.Outpoint) []*pb.Outpoint {
		if len(op) == 0 {
			return nil
		}
		ret := make([]*pb.Outpoint, len(op))
		for i, o := range op {
			ret[i] = &o
		}
		return ret
	}
	return brc, vrc, buyerAddr, vendorAddr, toPointer(buyerOutpointsOut), toPointer(vendorOutpointsOut), pb.OrderState(stateInt), nil
}
