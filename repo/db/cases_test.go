package db_test

import (
	"bytes"
	"database/sql"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmT6n4mspWYEya864BhCUJEgyxiRfmiSY9ruQwTUNpRKaM/protobuf/proto"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/golang/protobuf/ptypes"
)

func buildNewCaseStore() (repo.CaseStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return db.NewCaseStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestCasesDB_Count(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 5, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	i := casesdb.Count()
	if i != 1 {
		t.Error("Returned incorrect number of cases")
	}
}

func TestPutCase(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()

		caseID      string
		state       int
		read        int
		buyerOpened int
		claim       string
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := casesdb.PrepareQuery("select caseID, state, read, buyerOpened, claim from cases where caseID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()

	err = stmt.QueryRow("caseID").Scan(&caseID, &state, &read, &buyerOpened, &claim)
	if err != nil {
		t.Error(err)
	}
	if caseID != "caseID" {
		t.Errorf(`Expected %s got %s`, "caseID", caseID)
	}
	if state != 0 {
		t.Errorf(`Expected 0 got %d`, state)
	}
	if read != 0 {
		t.Errorf(`Expected 0 got %d`, read)
	}
	if buyerOpened != 1 {
		t.Errorf(`Expected 0 got %d`, buyerOpened)
	}
	if claim != strings.ToLower("blah") {
		t.Errorf(`Expected %s got %s`, strings.ToLower("blah"), claim)
	}
}

func TestUpdateWithNil(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID", nil, []string{"someError", "anotherError"}, "addr1", nil)
	if err != nil {
		t.Error(err)
	}
	dispute, err := casesdb.GetByCaseID("caseID")
	if err != nil {
		t.Error(err)
	}
	buyerContract, _, _, _, _, _, _, _, _, _, err := casesdb.GetCaseMetadata("caseID")
	if err != nil {
		t.Error(err)
	}
	if buyerContract != nil {
		t.Error("Vendor contract was not nil")
	}
	if dispute.BuyerOutpoints != nil {
		t.Error("Vendor outpoints was not nil")
	}
}

func TestDeleteCase(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.Delete("caseID")
	if err != nil {
		t.Error("Case delete failed")
	}

	stmt, _ := casesdb.PrepareQuery("select caseID from cases where caseID=?")
	defer stmt.Close()

	var caseID string
	err = stmt.QueryRow("caseID").Scan(&caseID)
	if err == nil {
		t.Error("Case delete failed")
	}
}

func TestMarkCaseAsRead(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.MarkAsRead("caseID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := casesdb.PrepareQuery("select read from cases where caseID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("caseID").Scan(&read)
	if err != nil {
		t.Error("Case query failed")
	}
	if read != 1 {
		t.Error("Failed to mark case as read")
	}
}

func TestMarkCaseAsUnread(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.MarkAsRead("caseID")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.MarkAsUnread("caseID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := casesdb.PrepareQuery("select read from cases where caseID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("caseID").Scan(&read)
	if err != nil {
		t.Error("Case query failed")
	}
	if read != 0 {
		t.Error("Failed to mark case as read")
	}
}

func TestUpdateBuyerInfo(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
		buyerTestOutpoints     = []*pb.Outpoint{{Hash: "hash1", Index: 0, Value: 5}}
		contract               = factory.NewContract()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID", contract, []string{"someError", "anotherError"}, "addr1", buyerTestOutpoints)
	if err != nil {
		t.Error(err)
	}

	stmt, err := casesdb.PrepareQuery("select caseID, buyerContract, buyerValidationErrors, buyerPayoutAddress, buyerOutpoints from cases where caseID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()

	var caseID string
	var buyerCon []byte
	var buyerErrors []byte
	var buyerAddr string
	var buyerOuts []byte
	err = stmt.QueryRow("caseID").Scan(&caseID, &buyerCon, &buyerErrors, &buyerAddr, &buyerOuts)
	if err != nil {
		t.Error(err)
	}
	if caseID != "caseID" {
		t.Errorf(`Expected %s got %s`, "caseID", caseID)
	}
	if len(buyerCon) <= 0 {
		t.Error(`Invalid contract returned`)
	}
	if buyerAddr != "addr1" {
		t.Errorf("Expected address %s got %s", "addr1", buyerAddr)
	}
	if string(buyerErrors) != `["someError","anotherError"]` {
		t.Errorf("Expected %s, got %s", `["someError","anotherError"]`, string(buyerErrors))
	}
	if string(buyerOuts) != `[{"hash":"hash1","value":5}]` {
		t.Errorf("Expected %s got %s", `[{"hash":"hash1","value":5}]`, string(buyerOuts))
	}
}

func TestUpdateVendorInfo(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
		contract               = factory.NewContract()
		vendorTestOutpoints    = []*pb.Outpoint{{Hash: "hash2", Index: 1, Value: 11}}
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 0, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateVendorInfo("caseID", contract, []string{"someError", "anotherError"}, "addr2", vendorTestOutpoints)
	if err != nil {
		t.Error(err)
	}

	stmt, err := casesdb.PrepareQuery("select caseID, vendorContract, vendorValidationErrors, vendorPayoutAddress, vendorOutpoints from cases where caseID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()

	var caseID string
	var vendorCon []byte
	var vendorErrors []byte
	var vendorAddr string
	var vendorOuts []byte
	err = stmt.QueryRow("caseID").Scan(&caseID, &vendorCon, &vendorErrors, &vendorAddr, &vendorOuts)
	if err != nil {
		t.Error(err)
	}
	if caseID != "caseID" {
		t.Errorf(`Expected %s got %s`, "caseID", caseID)
	}
	if len(vendorCon) <= 0 {
		t.Error(`Invalid contract returned`)
	}
	if vendorAddr != "addr2" {
		t.Errorf("Expected address %s got %s", "addr2", vendorAddr)
	}
	if string(vendorErrors) != `["someError","anotherError"]` {
		t.Errorf("Expected %s, got %s", `["someError","anotherError"]`, string(vendorErrors))
	}
	if string(vendorOuts) != `[{"hash":"hash2","index":1,"value":11}]` {
		t.Errorf("Expected %s got %s", `[{"hash":"hash2",index:1,value":11}]`, string(vendorOuts))
	}
}

func TestCasesGetCaseMetaData(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
		contract               = factory.NewContract()
		buyerTestOutpoints     = []*pb.Outpoint{{Hash: "hash1", Index: 0, Value: 5}}
		vendorTestOutpoints    = []*pb.Outpoint{{Hash: "hash2", Index: 1, Value: 11}}
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", pb.OrderState_DISPUTED, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID", contract, []string{"someError", "anotherError"}, "addr1", buyerTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateVendorInfo("caseID", contract, []string{"someError", "anotherError"}, "addr1", vendorTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, state, read, date, buyerOpened, claim, resolution, err := casesdb.GetCaseMetadata("caseID")
	if err != nil {
		t.Error(err)
	}
	ser, err := proto.Marshal(contract)
	if err != nil {
		t.Error(err)
	}
	buyerSer, err := proto.Marshal(buyerContract)
	if err != nil {
		t.Error(err)
	}
	vendorSer, err := proto.Marshal(vendorContract)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(ser, buyerSer) || !bytes.Equal(ser, vendorSer) {
		t.Error("Failed to fetch case contract from db")
	}
	if len(buyerValidationErrors) <= 0 || buyerValidationErrors[0] != "someError" || buyerValidationErrors[1] != "anotherError" {
		t.Error("Incorrect buyer validator errors")
	}
	if len(vendorValidationErrors) <= 0 || vendorValidationErrors[0] != "someError" || vendorValidationErrors[1] != "anotherError" {
		t.Error("Incorrect buyer validator errors")
	}
	if state != pb.OrderState_DISPUTED {
		t.Errorf("Expected state %s got %s", pb.OrderState_DISPUTED, state)
	}
	if read != false {
		t.Errorf("Expected read=%v got %v", false, read)
	}
	if date.After(time.Now()) || date.Equal(time.Time{}) {
		t.Error("Case timestamp invalid")
	}
	if !buyerOpened {
		t.Errorf("Expected buyerOpened=%v got %v", true, buyerOpened)
	}
	if claim != "blah" {
		t.Errorf("Expected claim=%s got %s", "blah", claim)
	}
	if resolution != nil {
		t.Error("Resolution should be nil")
	}
	_, _, _, _, _, _, _, _, _, _, err = casesdb.GetCaseMetadata("afasdfafd")
	if err == nil {
		t.Error("Get by unknown caseID failed to return error")
	}
}

func TestGetByCaseID(t *testing.T) {
	var (
		casesdb, teardown, err  = buildNewCaseStore()
		contract                = factory.NewContract()
		expectedBuyerOutpoints  = []*pb.Outpoint{{Hash: "hash1", Index: 0, Value: 5}}
		expectedVendorOutpoints = []*pb.Outpoint{{Hash: "hash2", Index: 1, Value: 11}}
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", pb.OrderState_DISPUTED, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID", contract, []string{"someError", "anotherError"}, "addr1", expectedBuyerOutpoints)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateVendorInfo("caseID", contract, []string{"someError", "anotherError"}, "addr2", expectedVendorOutpoints)
	if err != nil {
		t.Error(err)
	}

	dispute, err := casesdb.GetByCaseID("caseID")
	if err != nil {
		t.Error(err)
	}
	ser, _ := proto.Marshal(contract)
	buyerSer, _ := proto.Marshal(dispute.BuyerContract)
	vendorSer, _ := proto.Marshal(dispute.VendorContract)

	if !bytes.Equal(ser, buyerSer) || !bytes.Equal(ser, vendorSer) {
		t.Error("Failed to fetch case contract from db")
	}
	if dispute.BuyerPayoutAddress != "addr1" {
		t.Errorf("Expected address %s got %s", "addr1", dispute.BuyerPayoutAddress)
	}
	if dispute.VendorPayoutAddress != "addr2" {
		t.Errorf("Expected address %s got %s", "addr2", dispute.VendorPayoutAddress)
	}
	if len(dispute.BuyerOutpoints) != len(expectedBuyerOutpoints) {
		t.Error("Incorrect number of buyer outpoints returned")
	}
	for i, o := range dispute.BuyerOutpoints {
		if o.Hash != expectedBuyerOutpoints[i].Hash {
			t.Errorf("Expected outpoint hash %s got %s", o.Hash, expectedBuyerOutpoints[i].Hash)
		}
		if o.Index != expectedBuyerOutpoints[i].Index {
			t.Errorf("Expected outpoint index %v got %v", o.Index, expectedBuyerOutpoints[i].Index)
		}
		if o.Value != expectedBuyerOutpoints[i].Value {
			t.Errorf("Expected outpoint value %v got %v", o.Value, expectedBuyerOutpoints[i].Value)
		}
	}
	if len(dispute.VendorOutpoints) != len(expectedVendorOutpoints) {
		t.Error("Incorrect number of buyer outpoints returned")
	}
	for i, o := range expectedVendorOutpoints {
		if o.Hash != expectedVendorOutpoints[i].Hash {
			t.Errorf("Expected outpoint hash %s got %s", o.Hash, expectedVendorOutpoints[i].Hash)
		}
		if o.Index != expectedVendorOutpoints[i].Index {
			t.Errorf("Expected outpoint index %v got %v", o.Index, expectedVendorOutpoints[i].Index)
		}
		if o.Value != expectedVendorOutpoints[i].Value {
			t.Errorf("Expected outpoint value %v got %v", o.Value, expectedVendorOutpoints[i].Value)
		}
	}
	if dispute.OrderState != pb.OrderState_DISPUTED {
		t.Errorf("Expected state %s got %s", pb.OrderState_DISPUTED, dispute.OrderState)
	}
}

func TestMarkAsClosed(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
		contract               = factory.NewContract()
		buyerTestOutpoints     = []*pb.Outpoint{{Hash: "hash1", Index: 0, Value: 5}}
		vendorTestOutpoints    = []*pb.Outpoint{{Hash: "hash2", Index: 1, Value: 11}}
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", pb.OrderState_DISPUTED, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID", contract, []string{"someError", "anotherError"}, "addr1", buyerTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateVendorInfo("caseID", contract, []string{"someError", "anotherError"}, "addr2", vendorTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	d := new(pb.DisputeResolution)
	d.Resolution = "Case closed"
	err = casesdb.MarkAsClosed("caseID", d)
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, state, _, _, _, _, resolution, err := casesdb.GetCaseMetadata("caseID")
	if err != nil {
		t.Error(err)
	}
	if state != pb.OrderState_RESOLVED {
		t.Error("Mark as closed failed to set state to resolved")
	}
	if resolution.Resolution != d.Resolution {
		t.Error("Failed to save correct dispute resolution")
	}
}

func TestCasesDB_GetAll(t *testing.T) {
	var (
		casesdb, teardown, err = buildNewCaseStore()
		contract               = factory.NewContract()
		buyerTestOutpoints     = []*pb.Outpoint{{Hash: "hash1", Index: 0, Value: 5}}
		vendorTestOutpoints    = []*pb.Outpoint{{Hash: "hash2", Index: 1, Value: 11}}
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = casesdb.Put("caseID", 10, true, "blah", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID", contract, []string{"someError", "anotherError"}, "addr1", buyerTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateVendorInfo("caseID", contract, []string{"someError", "anotherError"}, "addr2", vendorTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	err = casesdb.Put("caseID2", 11, true, "asdf", "", "btc")
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateBuyerInfo("caseID2", contract, []string{"someError", "anotherError"}, "addr1", buyerTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.UpdateVendorInfo("caseID2", contract, []string{"someError", "anotherError"}, "addr2", vendorTestOutpoints)
	if err != nil {
		t.Error(err)
	}
	cases, ct, err := casesdb.GetAll([]pb.OrderState{}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 2 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query cases")
	}
	cases, ct, err = casesdb.GetAll([]pb.OrderState{}, "", false, false, 1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 1 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query cases")
	}
	cases, ct, err = casesdb.GetAll([]pb.OrderState{}, "", true, false, -1, []string{"caseID"})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 1 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query cases")
	}
	cases, ct, err = casesdb.GetAll([]pb.OrderState{pb.OrderState_DISPUTED}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 1 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 1 {
		t.Error("Returned incorrect number of query cases")
	}
	cases, ct, err = casesdb.GetAll([]pb.OrderState{pb.OrderState_DECIDED}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 1 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 1 {
		t.Error("Returned incorrect number of query cases")
	}
	cases, ct, err = casesdb.GetAll([]pb.OrderState{pb.OrderState_DISPUTED, pb.OrderState_DECIDED}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 2 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query cases")
	}
	cases, ct, err = casesdb.GetAll([]pb.OrderState{}, "caseid2", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(cases) != 1 {
		t.Error("Returned incorrect number of cases")
	}
	if ct != 1 {
		t.Error("Returned incorrect number of query cases")
	}
}

func TestGetDisputesForDisputeExpiryReturnsRelevantRecords(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	setupSQL := []string{
		schema.PragmaKey(""),
		schema.CreateTableDisputedCasesSQL,
	}
	_, err := database.Exec(strings.Join(setupSQL, " "))
	if err != nil {
		t.Fatal(err)
	}
	// Artificially start disputes 50 days ago
	var (
		now        = time.Unix(time.Now().Unix(), 0)
		timeStart  = now.Add(time.Duration(-50*24) * time.Hour)
		nowData, _ = ptypes.TimestampProto(now)
		order      = &pb.Order{
			BuyerID: &pb.ID{
				PeerID: "buyerID",
				Handle: "@buyerID",
			},
			Shipping: &pb.Order_Shipping{
				Address: "1234 Test Ave",
				ShipTo:  "Buyer Name",
			},
			Payment: &pb.Order_Payment{
				Amount:  10,
				Method:  pb.Order_Payment_DIRECT,
				Address: "3BDbGsH5h5ctDiFtWMmZawcf3E7iWirVms",
			},
			Timestamp: nowData,
		}
		expectedImagesOne = []*pb.Listing_Item_Image{{Tiny: "tinyimagehashOne", Small: "smallimagehashOne"}}
		contract          = &pb.RicardianContract{
			VendorListings: []*pb.Listing{
				{Item: &pb.Listing_Item{Images: expectedImagesOne}},
			},
			BuyerOrder: order,
		}
		neverNotified = &repo.DisputeCaseRecord{
			CaseID:                      "neverNotified",
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: time.Unix(0, 0),
			OrderState:                  pb.OrderState_DISPUTED,
			BuyerContract:               contract,
			VendorContract:              contract,
			IsBuyerInitiated:            true,
		}
		initialNotified = &repo.DisputeCaseRecord{
			CaseID:                      "initialNotificationSent",
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart,
			OrderState:                  pb.OrderState_DISPUTED,
			BuyerContract:               contract,
			VendorContract:              contract,
			IsBuyerInitiated:            true,
		}
		finallyNotified = &repo.DisputeCaseRecord{
			CaseID:                      "finalNotificationSent",
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: time.Now(),
			OrderState:                  pb.OrderState_DISPUTED,
			BuyerContract:               contract,
			VendorContract:              contract,
			IsBuyerInitiated:            true,
		}
		resolved = &repo.DisputeCaseRecord{
			CaseID:                      "resolved",
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart,
			OrderState:                  pb.OrderState_RESOLVED,
			BuyerContract:               contract,
			VendorContract:              contract,
		}
		existingRecords = []*repo.DisputeCaseRecord{
			neverNotified,
			initialNotified,
			finallyNotified,
			resolved,
		}
	)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	for _, r := range existingRecords {
		var isBuyerInitiated int = 0
		if r.IsBuyerInitiated {
			isBuyerInitiated = 1
		}
		buyerContract, err := m.MarshalToString(r.BuyerContract)
		if err != nil {
			t.Fatal(err)
		}
		vendorContract, err := m.MarshalToString(r.VendorContract)
		if err != nil {
			t.Fatal(err)
		}
		_, err = database.Exec("insert into cases (caseID, state, buyerContract, vendorContract, timestamp, buyerOpened, lastDisputeExpiryNotifiedAt) values (?, ?, ?, ?, ?, ?, ?);", r.CaseID, int(r.OrderState), buyerContract, vendorContract, int(r.Timestamp.Unix()), isBuyerInitiated, int(r.LastDisputeExpiryNotifiedAt.Unix()))
		if err != nil {
			t.Fatal(err)
		}
	}

	casesdb := db.NewCaseStore(database, new(sync.Mutex))
	cases, err := casesdb.GetDisputesForDisputeExpiryNotification()
	if err != nil {
		t.Fatal(err)
	}

	var (
		sawNeverNotifiedCase   bool
		sawInitialNotifiedCase bool
		sawFinallyNotifiedCase bool
		sawResolvedCase        bool
	)
	for _, c := range cases {
		switch c.CaseID {
		case neverNotified.CaseID:
			sawNeverNotifiedCase = true
			if reflect.DeepEqual(c, neverNotified) != true {
				t.Error("Expected neverNotified to match, but did not")
				t.Error("Expected:", neverNotified)
				t.Error("Actual:", c)
			}
		case initialNotified.CaseID:
			sawInitialNotifiedCase = true
			if reflect.DeepEqual(c, initialNotified) != true {
				t.Error("Expected initialNotified to match, but did not")
				t.Error("Expected:", initialNotified)
				t.Error("Actual:", c)
			}
		case finallyNotified.CaseID:
			sawFinallyNotifiedCase = true
		case resolved.CaseID:
			sawResolvedCase = true
		default:
			t.Errorf("Found unexpected dispute case: %+v", c)
		}
	}

	if sawNeverNotifiedCase == false {
		t.Error("Expected to see case which was never notified")
	}
	if sawInitialNotifiedCase == false {
		t.Error("Expected to see case which was initially notified")
	}
	if sawFinallyNotifiedCase == true {
		t.Error("Expected NOT to see case which recieved it's final notification")
	}
	if sawResolvedCase == true {
		t.Error("Expected NOT to see case which is resolved")
	}
}

func TestGetDisputesForDisputeExpiryAllowsMissingContracts(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	setupSQL := []string{
		schema.PragmaKey(""),
		schema.CreateTableDisputedCasesSQL,
	}
	_, err := database.Exec(strings.Join(setupSQL, " "))
	if err != nil {
		t.Fatal(err)
	}
	var (
		now        = time.Unix(time.Now().Unix(), 0)
		timeStart  = now.Add(time.Duration(-50*24) * time.Hour)
		nowData, _ = ptypes.TimestampProto(now)
		order      = &pb.Order{
			BuyerID: &pb.ID{
				PeerID: "buyerID",
				Handle: "@buyerID",
			},
			Shipping: &pb.Order_Shipping{
				Address: "1234 Test Ave",
				ShipTo:  "Buyer Name",
			},
			Payment: &pb.Order_Payment{
				Amount:  10,
				Method:  pb.Order_Payment_DIRECT,
				Address: "3BDbGsH5h5ctDiFtWMmZawcf3E7iWirVms",
			},
			Timestamp: nowData,
		}
		expectedImagesOne = []*pb.Listing_Item_Image{{Tiny: "tinyimagehashOne", Small: "smallimagehashOne"}}
		contract          = &pb.RicardianContract{
			VendorListings: []*pb.Listing{
				{Item: &pb.Listing_Item{Images: expectedImagesOne}},
			},
			BuyerOrder: order,
		}
		missingVendorContract = &repo.DisputeCaseRecord{
			CaseID:                      "neverNotified",
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: time.Unix(0, 0),
			BuyerContract:               contract,
			IsBuyerInitiated:            true,
		}
		missingBuyerContract = &repo.DisputeCaseRecord{
			CaseID:                      "initialNotificationSent",
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart,
			VendorContract:              contract,
			IsBuyerInitiated:            true,
		}
		existingRecords = []*repo.DisputeCaseRecord{
			missingVendorContract,
			missingBuyerContract,
		}
	)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	for _, r := range existingRecords {
		var (
			isBuyerInitiated int
			buyerContract    = sql.NullString{}
			vendorContract   = sql.NullString{}
			err              error
		)

		if r.IsBuyerInitiated {
			isBuyerInitiated = 1
		}
		if r.BuyerContract != nil {
			buyerContract.String, err = m.MarshalToString(r.BuyerContract)
			if err != nil {
				t.Fatal(err)
			}
			buyerContract.Valid = true
		}
		if r.VendorContract != nil {
			vendorContract.String, err = m.MarshalToString(r.VendorContract)
			if err != nil {
				t.Fatal(err)
			}
			vendorContract.Valid = true
		}
		_, err = database.Exec("insert into cases (caseID, buyerContract, vendorContract, timestamp, buyerOpened, lastDisputeExpiryNotifiedAt) values (?, ?, ?, ?, ?, ?);", r.CaseID, buyerContract, vendorContract, int(r.Timestamp.Unix()), isBuyerInitiated, int(r.LastDisputeExpiryNotifiedAt.Unix()))
		if err != nil {
			t.Fatal(err)
		}
	}

	casesdb := db.NewCaseStore(database, new(sync.Mutex))
	_, err = casesdb.GetDisputesForDisputeExpiryNotification()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDisputeLastDisputeExpiryNotifiedAt(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	setupSQL := []string{
		schema.PragmaKey(""),
		schema.CreateTableDisputedCasesSQL,
	}
	_, err := database.Exec(strings.Join(setupSQL, " "))
	if err != nil {
		t.Fatal(err)
	}
	// Artificially start disputes 50 days ago
	timeStart := time.Now().Add(time.Duration(-50*24) * time.Hour)
	disputeOne := &repo.DisputeCaseRecord{
		CaseID:                      "case1",
		Timestamp:                   timeStart,
		LastDisputeExpiryNotifiedAt: time.Unix(123, 0),
	}
	disputeTwo := &repo.DisputeCaseRecord{
		CaseID:                      "case2",
		Timestamp:                   timeStart,
		LastDisputeExpiryNotifiedAt: time.Unix(456, 0),
	}
	_, err = database.Exec("insert into cases (caseID, timestamp, lastDisputeExpiryNotifiedAt) values (?, ?, ?);", disputeOne.CaseID, disputeOne.Timestamp, disputeOne.LastDisputeExpiryNotifiedAt)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec("insert into cases (caseID, timestamp, lastDisputeExpiryNotifiedAt) values (?, ?, ?);", disputeTwo.CaseID, int(disputeTwo.Timestamp.Unix()), int(disputeTwo.LastDisputeExpiryNotifiedAt.Unix()))
	if err != nil {
		t.Fatal(err)
	}

	disputeOne.LastDisputeExpiryNotifiedAt = time.Unix(987, 0)
	disputeTwo.LastDisputeExpiryNotifiedAt = time.Unix(765, 0)
	casesdb := db.NewCaseStore(database, new(sync.Mutex))
	err = casesdb.UpdateDisputesLastDisputeExpiryNotifiedAt([]*repo.DisputeCaseRecord{disputeOne, disputeTwo})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := database.Query("select caseID, lastDisputeExpiryNotifiedAt from cases")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			caseID                      string
			lastDisputeExpiryNotifiedAt int64
		)
		if err = rows.Scan(&caseID, &lastDisputeExpiryNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch caseID {
		case disputeOne.CaseID:
			if time.Unix(lastDisputeExpiryNotifiedAt, 0).Equal(disputeOne.LastDisputeExpiryNotifiedAt) != true {
				t.Error("Expected disputeOne.LastDisputeExpiryNotifiedAt to be updated")
			}
		case disputeTwo.CaseID:
			if time.Unix(lastDisputeExpiryNotifiedAt, 0).Equal(disputeTwo.LastDisputeExpiryNotifiedAt) != true {
				t.Error("Expected disputeTwo.LastDisputeExpiryNotifiedAt to be updated")
			}
		default:
			t.Error("Unexpected dispute case encounted")
			t.Error(caseID, lastDisputeExpiryNotifiedAt)
		}

	}
}

func TestCasesDB_Put_PaymentCoin(t *testing.T) {

	var (
		tests = []struct {
			acceptedCurrencies []string
			paymentCoin        string
			expected           string
		}{
			{[]string{"TBTC"}, "TBTC", "TBTC"},
			{[]string{"TBTC", "TBCH"}, "TBTC", "TBTC"},
			{[]string{"TBCH", "TBTC"}, "TBTC", "TBTC"},
			{[]string{"TBTC", "TBCH"}, "TBCH", "TBCH"},
			{[]string{}, "", ""},
		}
		contract = factory.NewContract()
	)

	for _, test := range tests {
		var casesdb, teardown, err = buildNewCaseStore()
		if err != nil {
			t.Fatal(err)
		}

		contract.VendorListings[0].Metadata.AcceptedCurrencies = test.acceptedCurrencies
		contract.BuyerOrder.Payment.Coin = test.paymentCoin

		err = casesdb.PutRecord(&repo.DisputeCaseRecord{
			CaseID:           "paymentCoinTest",
			BuyerContract:    contract,
			VendorContract:   contract,
			IsBuyerInitiated: true,
			PaymentCoin:      test.paymentCoin,
		})
		if err != nil {
			t.Error(err)
		}

		cases, count, err := casesdb.GetAll(nil, "", false, false, 1, nil)
		if err != nil {
			t.Error(err)
		}
		if count != 1 {
			t.Errorf(`Expected %d record got %d`, 1, count)
		}
		if cases[0].PaymentCoin != test.expected {
			t.Errorf(`Expected %s got %s`, test.expected, cases[0].PaymentCoin)
		}
		teardown()
	}
}

func TestCasesDB_Put_CoinType(t *testing.T) {
	var (
		testsCoins = []string{"", "TBTC", "TETH"}
		contract   = factory.NewContract()
	)

	for _, testCoin := range testsCoins {
		var casesdb, teardown, err = buildNewCaseStore()
		if err != nil {
			t.Fatal(err)
		}

		contract.VendorListings[0].Metadata.CoinType = testCoin

		err = casesdb.PutRecord(&repo.DisputeCaseRecord{
			CaseID:           "paymentCoinTest",
			BuyerContract:    contract,
			VendorContract:   contract,
			IsBuyerInitiated: true,
			CoinType:         testCoin,
		})
		if err != nil {
			t.Error(err)
		}

		cases, count, err := casesdb.GetAll(nil, "", false, false, 1, nil)
		if err != nil {
			t.Error(err)
		}
		if count != 1 {
			t.Errorf(`Expected %d record got %d`, 1, count)
		}
		if cases[0].CoinType != testCoin {
			t.Errorf(`Expected %s got %s`, testCoin, cases[0].CoinType)
		}
		err = casesdb.Delete(cases[0].CaseId)
		if err != nil {
			t.Error("Sale delete failed")
		}
		teardown()
	}
}
