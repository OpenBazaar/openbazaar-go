package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type CasesDB struct {
	modelStore
}

func NewCaseStore(db *sql.DB, lock *sync.Mutex) repo.CaseStore {
	return &CasesDB{modelStore{db, lock}}
}

func (c *CasesDB) PutRecord(dispute *repo.DisputeCaseRecord) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var readInt, buyerOpenedInt uint
	if dispute.IsBuyerInitiated {
		buyerOpenedInt = 1
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into cases(caseID, state, read, timestamp, buyerOpened, claim, buyerPayoutAddress, vendorPayoutAddress, paymentCoin, coinType) values(?,?,?,?,?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}

	contract := contractForDispute(dispute)
	var coinType, paymentCoin string
	if contract != nil {
		coinType = coinTypeForContract(contract)
		paymentCoin = paymentCoinForContract(contract)
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		dispute.CaseID,
		int(dispute.OrderState),
		readInt,
		int(dispute.Timestamp.Unix()),
		buyerOpenedInt,
		dispute.Claim,
		"",
		"",
		paymentCoin,
		coinType,
	)
	if err != nil {
		rErr := tx.Rollback()
		if rErr != nil {
			return fmt.Errorf("case put fail: %s\nand rollback failed: %s\n", err.Error(), rErr.Error())
		}
		return err
	}

	return tx.Commit()
}

func (c *CasesDB) Put(caseID string, state pb.OrderState, buyerOpened bool, claim string) error {
	record := &repo.DisputeCaseRecord{
		CaseID:           caseID,
		Claim:            claim,
		IsBuyerInitiated: buyerOpened,
		OrderState:       state,
		Timestamp:        time.Now(),
	}
	fmt.Printf("Put Record: %+v\n", record)
	return c.PutRecord(record)
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

func (c *CasesDB) MarkAsUnread(orderID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("update cases set read=? where caseID=?", 0, orderID)
	if err != nil {
		return err
	}
	return nil
}

func (c *CasesDB) MarkAsClosed(caseID string, resolution *pb.DisputeResolution) error {
	if resolution == nil {
		return errors.New("Dispute resolution should not be nil")
	}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	var rOut string
	var err error
	rOut, err = m.MarshalToString(resolution)
	if err != nil {
		return err
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

func (c *CasesDB) GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]repo.Case, int, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	q := query{
		table:           "cases",
		columns:         []string{"caseID", "timestamp", "buyerContract", "vendorContract", "buyerOpened", "state", "read", "coinType", "paymentCoin"},
		stateFilter:     stateFilter,
		searchTerm:      searchTerm,
		searchColumns:   []string{"caseID", "timestamp", "claim"},
		sortByAscending: sortByAscending,
		sortByRead:      sortByRead,
		id:              "caseID",
		exclude:         exclude,
		limit:           limit,
	}
	stm, args := filterQuery(q)
	rows, err := c.db.Query(stm, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var ret []repo.Case
	for rows.Next() {
		var caseID, coinType, paymentCoin string
		var buyerContract, vendorContract []byte
		var timestamp, buyerOpenedInt, stateInt, readInt int
		if err := rows.Scan(&caseID, &timestamp, &buyerContract, &vendorContract, &buyerOpenedInt, &stateInt, &readInt, &coinType, &paymentCoin); err != nil {
			return ret, 0, err
		}
		read := false
		if readInt > 0 {
			read = true
		}

		buyerOpened := false
		if buyerOpenedInt > 0 {
			buyerOpened = true
		}
		var total uint64
		var title, thumbnail, vendorId, vendorHandle, buyerId, buyerHandle string

		contract := new(pb.RicardianContract)
		err := jsonpb.UnmarshalString(string(buyerContract), contract)
		if err != nil {
			jsonpb.UnmarshalString(string(vendorContract), contract)
		}
		var slug string
		if contract != nil {
			if len(contract.VendorListings) > 0 {
				slug = contract.VendorListings[0].Slug
				if contract.VendorListings[0].VendorID != nil {
					vendorId = contract.VendorListings[0].VendorID.PeerID
					vendorHandle = contract.VendorListings[0].VendorID.Handle
				}
				if contract.VendorListings[0].Item != nil {
					title = contract.VendorListings[0].Item.Title
					if len(contract.VendorListings[0].Item.Images) > 0 {
						thumbnail = contract.VendorListings[0].Item.Images[0].Tiny
					}
				}
			}
			if contract.BuyerOrder != nil {
				slug = contract.VendorListings[0].Slug
				if contract.BuyerOrder.BuyerID != nil {
					buyerId = contract.BuyerOrder.BuyerID.PeerID
					buyerHandle = contract.BuyerOrder.BuyerID.Handle
				}
				if contract.BuyerOrder.Payment != nil {
					total = contract.BuyerOrder.Payment.Amount
				}
			}
		}

		ret = append(ret, repo.Case{
			CaseId:       caseID,
			Slug:         slug,
			Timestamp:    time.Unix(int64(timestamp), 0),
			Title:        title,
			Thumbnail:    thumbnail,
			Total:        total,
			VendorId:     vendorId,
			VendorHandle: vendorHandle,
			BuyerId:      buyerId,
			BuyerHandle:  buyerHandle,
			BuyerOpened:  buyerOpened,
			CoinType:     coinType,
			PaymentCoin:  paymentCoin,
			State:        pb.OrderState(stateInt).String(),
			Read:         read,
		})
	}
	q.columns = []string{"Count(*)"}
	q.limit = -1
	q.exclude = []string{}
	stm, args = filterQuery(q)
	row := c.db.QueryRow(stm, args...)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return ret, 0, err
	}
	return ret, count, nil
}

func (c *CasesDB) GetCaseMetadata(caseID string) (buyerContract, vendorContract *pb.RicardianContract, buyerValidationErrors, vendorValidationErrors []string, state pb.OrderState, read bool, timestamp time.Time, buyerOpened bool, claim string, resolution *pb.DisputeResolution, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stmt, err := c.db.Prepare("select buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, state, read, timestamp, buyerOpened, claim, disputeResolution from cases where caseID=?")
	defer stmt.Close()
	var buyerCon []byte
	var vendorCon []byte
	var buyerErrors []byte
	var vendorErrors []byte
	var stateInt int
	var readInt *int
	var ts int
	var buyerOpenedInt int
	var disputResolution []byte
	err = stmt.QueryRow(caseID).Scan(&buyerCon, &vendorCon, &buyerErrors, &vendorErrors, &stateInt, &readInt, &ts, &buyerOpenedInt, &claim, &disputResolution)
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
	if string(buyerErrors) != "" {
		err = json.Unmarshal(buyerErrors, &berr)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
		}
	}
	var verr []string
	if string(vendorErrors) != "" {
		err = json.Unmarshal(vendorErrors, &verr)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
		}
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

	return brc, vrc, berr, verr, pb.OrderState(stateInt), read, time.Unix(int64(ts), 0), buyerOpened, claim, resolution, nil
}

func (c *CasesDB) GetByCaseID(caseID string) (*repo.DisputeCaseRecord, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var buyerCon []byte
	var vendorCon []byte
	var buyerOuts []byte
	var vendorOuts []byte
	var buyerAddr string
	var vendorAddr string
	var stateInt int
	var isBuyerInitiated int
	var buyerInitiated bool
	var createdAt int64

	stmt, err := c.db.Prepare("select buyerContract, vendorContract, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, buyerOpened, timestamp from cases where caseID=?")
	if err != nil {
		return nil, err
	}
	err = stmt.QueryRow(caseID).Scan(&buyerCon, &vendorCon, &buyerAddr, &vendorAddr, &buyerOuts, &vendorOuts, &stateInt, &isBuyerInitiated, &createdAt)
	if err != nil {
		return nil, err
	}

	if isBuyerInitiated == 1 {
		buyerInitiated = true
	}

	brc := new(pb.RicardianContract)
	if string(buyerCon) != "" {
		err = jsonpb.UnmarshalString(string(buyerCon), brc)
		if err != nil {
			return nil, err
		}
	} else {
		brc = nil
	}
	vrc := new(pb.RicardianContract)
	if string(vendorCon) != "" {
		err = jsonpb.UnmarshalString(string(vendorCon), vrc)
		if err != nil {
			return nil, err
		}
	} else {
		vrc = nil
	}

	var buyerOutpointsOut []pb.Outpoint
	if len(buyerOuts) > 0 {
		err = json.Unmarshal(buyerOuts, &buyerOutpointsOut)
		if err != nil {
			return nil, err
		}
	}
	var vendorOutpointsOut []pb.Outpoint
	if len(vendorOuts) > 0 {
		err = json.Unmarshal(vendorOuts, &vendorOutpointsOut)
		if err != nil {
			return nil, err
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
	return &repo.DisputeCaseRecord{
		CaseID:              caseID,
		IsBuyerInitiated:    buyerInitiated,
		BuyerContract:       brc,
		BuyerPayoutAddress:  buyerAddr,
		BuyerOutpoints:      toPointer(buyerOutpointsOut),
		VendorContract:      vrc,
		VendorPayoutAddress: vendorAddr,
		VendorOutpoints:     toPointer(vendorOutpointsOut),
		OrderState:          pb.OrderState(stateInt),
		Timestamp:           time.Unix(createdAt, 0),
	}, nil
}

func (c *CasesDB) Count() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	row := c.db.QueryRow("select Count(*) from cases")
	var count int
	row.Scan(&count)
	return count
}

// GetDisputesForDisputeExpiryNotification returns []*repo.DisputeCaseRecord including
// each record which needs Notifications to be generated. Currently,
// notifications are generated at 0, 15, 30, 44, and 45 days after opening.
func (c *CasesDB) GetDisputesForDisputeExpiryNotification() ([]*repo.DisputeCaseRecord, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	rows, err := c.db.Query("select caseID, buyerContract, vendorContract, timestamp, buyerOpened, lastDisputeExpiryNotifiedAt from cases where (lastDisputeExpiryNotifiedAt - timestamp) < ?",
		int(repo.ModeratorDisputeExpiry_lastInterval.Seconds()),
	)
	if err != nil {
		return nil, fmt.Errorf("selecting dispute case: %s", err.Error())
	}
	result := make([]*repo.DisputeCaseRecord, 0)
	for rows.Next() {
		var (
			lastDisputeExpiryNotifiedAt   int64
			isBuyerInitiated              int
			buyerContract, vendorContract []byte

			r = &repo.DisputeCaseRecord{
				BuyerContract:  &pb.RicardianContract{},
				VendorContract: &pb.RicardianContract{},
			}
			timestamp = sql.NullInt64{}
		)
		if err := rows.Scan(&r.CaseID, &buyerContract, &vendorContract, &timestamp, &isBuyerInitiated, &lastDisputeExpiryNotifiedAt); err != nil {
			return nil, fmt.Errorf("scanning dispute case: %s", err.Error())
		}
		if len(buyerContract) > 0 {
			if err := jsonpb.UnmarshalString(string(buyerContract), r.BuyerContract); err != nil {
				return nil, fmt.Errorf("unmarshaling buyer contract: %s\n", err.Error())
			}
		}
		if len(vendorContract) > 0 {
			if err := jsonpb.UnmarshalString(string(vendorContract), r.VendorContract); err != nil {
				return nil, fmt.Errorf("unmarshaling vendor contract: %s\n", err.Error())
			}
		}
		if isBuyerInitiated != 0 {
			r.IsBuyerInitiated = true
		}
		if timestamp.Valid {
			r.Timestamp = time.Unix(timestamp.Int64, 0)
		} else {
			r.Timestamp = time.Now()
		}
		r.LastDisputeExpiryNotifiedAt = time.Unix(lastDisputeExpiryNotifiedAt, 0)
		result = append(result, r)
	}
	return result, nil
}

// UpdateDisputesLastDisputeExpiryNotifiedAt accepts []*repo.DisputeCaseRecord and updates
// each DisputeCaseRecord by their CaseID to the set LastDisputeExpiryNotifiedAt value. The
// update will be attempted atomically with a rollback attempted in the event of
// an error.
func (c *CasesDB) UpdateDisputesLastDisputeExpiryNotifiedAt(disputeCases []*repo.DisputeCaseRecord) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin update disputes transaction: %s", err.Error())
	}
	for _, d := range disputeCases {
		_, err = tx.Exec("update cases set lastDisputeExpiryNotifiedAt = ? where caseID = ?", int(d.LastDisputeExpiryNotifiedAt.Unix()), d.CaseID)
		if err != nil {
			return fmt.Errorf("update dispute case: %s", err.Error())
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit update disputes transaction: %s", err.Error())
	}

	return nil
}
