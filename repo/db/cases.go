package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
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

	if dispute.PaymentCoin.String() == "" {
		return errors.New("payment coin field is empty")
	}

	var readInt, buyerOpenedInt uint
	if dispute.IsBuyerInitiated {
		buyerOpenedInt = 1
	}

	stm := `insert or replace into cases(caseID, state, read, timestamp, buyerOpened, claim, buyerPayoutAddress, vendorPayoutAddress, paymentCoin, coinType) values(?,?,?,?,?,?,?,?,?,?)`
	stmt, err := c.PrepareQuery(stm)
	if err != nil {
		return err
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
		dispute.PaymentCoin.String(),
		dispute.CoinType,
	)
	if err != nil {
		return fmt.Errorf("update dispute case: %s", err.Error())
	}

	return nil
}

func (c *CasesDB) Put(caseID string, state pb.OrderState, buyerOpened bool, claim string, paymentCoin string, coinType string) error {
	def, err := repo.AllCurrencies().Lookup(paymentCoin)
	if err != nil {
		return fmt.Errorf("verifying paymentCoin: %s", err.Error())
	}
	cc := def.CurrencyCode()
	record := &repo.DisputeCaseRecord{
		CaseID:           caseID,
		Claim:            claim,
		IsBuyerInitiated: buyerOpened,
		OrderState:       state,
		PaymentCoin:      &cc,
		CoinType:         coinType,
		Timestamp:        time.Now(),
	}
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
		return errors.New("dispute resolution should not be nil")
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
		total := new(big.Int)
		var title, thumbnail, vendorId, vendorHandle, buyerId, buyerHandle string

		contract := new(pb.RicardianContract)
		err := jsonpb.UnmarshalString(string(buyerContract), contract)
		if err != nil {
			err = jsonpb.UnmarshalString(string(vendorContract), contract)
			if err != nil {
				log.Errorf("Error unmarshaling case contract: %s", err)
				continue
			}
		}
		var slug string
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

			if contract.VendorListings[0].Metadata != nil && contract.VendorListings[0].Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY {
				coinType = ""
			}
		}
		if contract.BuyerOrder != nil {
			slug = contract.VendorListings[0].Slug
			if contract.BuyerOrder.BuyerID != nil {
				buyerId = contract.BuyerOrder.BuyerID.PeerID
				buyerHandle = contract.BuyerOrder.BuyerID.Handle
			}
			if contract.BuyerOrder.Payment != nil {
				if contract.BuyerOrder.Payment.BigAmount != "" {
					total0, _ := new(big.Int).SetString(contract.BuyerOrder.Payment.BigAmount, 10)
					total = total0
				} else {
					total1 := new(big.Int).SetUint64(contract.BuyerOrder.Payment.Amount)
					total = total1
				}
			}
		}

		cv, err := repo.NewCurrencyValueWithLookup(total.String(), paymentCoin)
		if err != nil {
			return nil, 0, err
		}

		ret = append(ret, repo.Case{
			CaseId:       caseID,
			Slug:         slug,
			Timestamp:    time.Unix(int64(timestamp), 0),
			Title:        title,
			Thumbnail:    thumbnail,
			Total:        *cv,
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
	stmt, err := c.PrepareQuery("select buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, state, read, timestamp, buyerOpened, claim, disputeResolution from cases where caseID=?")
	if err != nil {
		return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, err
	}
	defer stmt.Close()

	var (
		buyerCon          []byte
		vendorCon         []byte
		buyerErrors       []byte
		vendorErrors      []byte
		stateInt          int
		readInt           *int
		ts                int
		buyerOpenedInt    int
		disputeResolution []byte
	)
	err = stmt.QueryRow(caseID).Scan(&buyerCon, &vendorCon, &buyerErrors, &vendorErrors, &stateInt, &readInt, &ts, &buyerOpenedInt, &claim, &disputeResolution)
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
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, fmt.Errorf("unmarshal dispute case: %s", err.Error())
		}
	}
	var verr []string
	if string(vendorErrors) != "" {
		err = json.Unmarshal(vendorErrors, &verr)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, fmt.Errorf("unmarshal dispute vendor errors: %s", err.Error())
		}
	}
	resolution = new(pb.DisputeResolution)
	if string(disputeResolution) != "" {
		err = jsonpb.UnmarshalString(string(disputeResolution), resolution)
		if err != nil {
			return nil, nil, []string{}, []string{}, pb.OrderState(0), false, time.Time{}, false, "", nil, fmt.Errorf("unmarhsal dispute case resolution: %s", err.Error())
		}
	} else {
		resolution = nil
	}

	return brc, vrc, berr, verr, pb.OrderState(stateInt), read, time.Unix(int64(ts), 0), buyerOpened, claim, resolution, nil
}

func (c *CasesDB) GetByCaseID(caseID string) (*repo.DisputeCaseRecord, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var (
		buyerAddr        string
		buyerCon         []byte
		buyerInitiated   bool
		buyerOuts        []byte
		createdAt        int64
		isBuyerInitiated int
		paymentCoin      string
		stateInt         int
		vendorAddr       string
		vendorCon        []byte
		vendorOuts       []byte
	)

	stmt, err := c.PrepareQuery("select buyerContract, vendorContract, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, buyerOpened, timestamp, paymentCoin from cases where caseID=?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(caseID).Scan(&buyerCon, &vendorCon, &buyerAddr, &vendorAddr, &buyerOuts, &vendorOuts, &stateInt, &isBuyerInitiated, &createdAt, &paymentCoin)
	if err != nil {
		return nil, err
	}

	def, err := repo.AllCurrencies().Lookup(paymentCoin)
	if err != nil {
		return nil, fmt.Errorf("validating payment coin: %s", err.Error())
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
			var newOutpoint = o
			ret[i] = &newOutpoint
		}
		return ret
	}
	cc := def.CurrencyCode()
	return &repo.DisputeCaseRecord{
		BuyerContract:       brc,
		BuyerOutpoints:      toPointer(buyerOutpointsOut),
		BuyerPayoutAddress:  buyerAddr,
		CaseID:              caseID,
		IsBuyerInitiated:    buyerInitiated,
		OrderState:          pb.OrderState(stateInt),
		PaymentCoin:         &cc,
		Timestamp:           time.Unix(createdAt, 0),
		VendorContract:      vrc,
		VendorOutpoints:     toPointer(vendorOutpointsOut),
		VendorPayoutAddress: vendorAddr,
	}, nil
}

func (c *CasesDB) Count() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	row := c.db.QueryRow("select Count(*) from cases")
	var count int
	err := row.Scan(&count)
	if err != nil {
		log.Error(err)
	}
	return count
}

// GetDisputesForDisputeExpiryNotification returns []*repo.DisputeCaseRecord including
// each record which needs Notifications to be generated. Currently,
// notifications are generated at 0, 15, 30, 44, and 45 days after opening.
func (c *CasesDB) GetDisputesForDisputeExpiryNotification() ([]*repo.DisputeCaseRecord, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	rows, err := c.db.Query("select caseID, state, buyerContract, vendorContract, timestamp, buyerOpened, lastDisputeExpiryNotifiedAt from cases where (lastDisputeExpiryNotifiedAt - timestamp) < ? and state = ?",
		int(repo.ModeratorDisputeExpiry_lastInterval.Seconds()),
		int(pb.OrderState_DISPUTED),
	)
	if err != nil {
		return nil, fmt.Errorf("selecting dispute case: %s", err.Error())
	}
	result := make([]*repo.DisputeCaseRecord, 0)
	for rows.Next() {
		var (
			orderState                    int
			lastDisputeExpiryNotifiedAt   int64
			isBuyerInitiated              int
			buyerContract, vendorContract []byte

			r = &repo.DisputeCaseRecord{
				BuyerContract:  &pb.RicardianContract{},
				VendorContract: &pb.RicardianContract{},
			}
			timestamp = sql.NullInt64{}
		)
		if err := rows.Scan(&r.CaseID, &orderState, &buyerContract, &vendorContract, &timestamp, &isBuyerInitiated, &lastDisputeExpiryNotifiedAt); err != nil {
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
		r.OrderState = pb.OrderState(orderState)
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

	tx, err := c.BeginTransaction()
	if err != nil {
		return fmt.Errorf("begin update disputes transaction: %s", err.Error())
	}
	for _, d := range disputeCases {
		_, err = tx.Exec("update cases set lastDisputeExpiryNotifiedAt = ? where caseID = ?", int(d.LastDisputeExpiryNotifiedAt.Unix()), d.CaseID)
		if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				return fmt.Errorf("update dispute case: (%s) w rollback error: (%s)", err.Error(), rErr.Error())
			}
			return fmt.Errorf("update dispute case: %s", err.Error())
		}
	}
	if err = tx.Commit(); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return fmt.Errorf("commit dispute case: (%s) w rollback error: (%s)", err.Error(), rErr.Error())
		}
		return fmt.Errorf("commit disputes case: %s", err.Error())
	}

	return nil
}
