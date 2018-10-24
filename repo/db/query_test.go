package db

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

func Test_filterQuery(t *testing.T) {

	// Test search term
	stm, args := filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{},
		searchTerm:      "test",
		searchColumns:   []string{"orderID", "timestamp", "title"},
		id:              "orderID",
		sortByAscending: false,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where (orderID || timestamp || title) like ? order by timestamp desc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 1 {
		t.Error("Incorrect args")
	}

	// Test excluded ids
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{},
		searchTerm:      "",
		searchColumns:   []string{},
		id:              "orderID",
		exclude:         []string{"abc", "xyz"},
		sortByAscending: false,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where orderID not in (?,?) order by timestamp desc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 2 {
		t.Error("Incorrect args")
	}

	// Test state filter
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING, pb.OrderState_AWAITING_PAYMENT},
		searchTerm:      "",
		searchColumns:   []string{},
		id:              "orderID",
		exclude:         []string{},
		sortByAscending: false,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where state in (?,?) order by timestamp desc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 2 {
		t.Error("Incorrect args")
	}

	// Test ascending
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING, pb.OrderState_AWAITING_PAYMENT},
		searchTerm:      "",
		searchColumns:   []string{},
		id:              "orderID",
		exclude:         []string{},
		sortByAscending: true,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where state in (?,?) order by timestamp asc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 2 {
		t.Error("Incorrect args")
	}

	// Test sort by read
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING, pb.OrderState_AWAITING_PAYMENT},
		searchTerm:      "",
		searchColumns:   []string{},
		id:              "orderID",
		exclude:         []string{},
		sortByAscending: true,
		sortByRead:      true,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where state in (?,?) order by read asc, timestamp asc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 2 {
		t.Error("Incorrect args")
	}

	// Test state filter and exclude
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING, pb.OrderState_AWAITING_PAYMENT},
		searchTerm:      "",
		searchColumns:   []string{},
		id:              "orderID",
		exclude:         []string{"abc", "xyz"},
		sortByAscending: false,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where state in (?,?) and orderID not in (?,?) order by timestamp desc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 4 {
		t.Error("Incorrect args")
	}

	// Test search and state filter
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING, pb.OrderState_AWAITING_PAYMENT},
		searchTerm:      "hello",
		searchColumns:   []string{"orderID", "timestamp", "title"},
		id:              "orderID",
		exclude:         []string{},
		sortByAscending: false,
		limit:           -1,
	})
	if stm != "select orderID, timestamp from purchases where state in (?,?) and (orderID || timestamp || title) like ? order by timestamp desc limit -1;" {
		t.Error("Incorrect statement")
	}
	if len(args) != 3 {
		t.Error("Incorrect args")
	}

}
