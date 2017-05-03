package db

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func Test_filterQuery(t *testing.T) {
	stm, args := filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offset:          5,
		stateFilter:     []pb.OrderState{},
		searchTerm:      "test",
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: false,
		limit:           -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp desc limit -1 offset 5;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 1 {
		t.Error("Incorrect number of args returned")
	}

	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offset:          0,
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING},
		searchTerm:      "test",
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: true,
		limit:           -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where state in (?) and (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 2 {
		t.Error("Incorrect number of args returned")
	}
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING},
		searchTerm:      "test",
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: true,
		sortByRead:      true,
		offset:          10,
		limit:           -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where state in (?) and (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by read asc, timestamp asc limit -1 offset 10;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 2 {
		t.Error("Incorrect number of args returned")
	}
	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		stateFilter:     []pb.OrderState{pb.OrderState_PENDING},
		searchTerm:      "",
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: true,
		limit:           -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where state in (?) order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 1 {
		t.Error("Incorrect number of args returned")
	}

	stm, args = filterQuery(query{
		table:           "purchases",
		columns:         []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		stateFilter:     []pb.OrderState{},
		searchTerm:      "asdf",
		searchColumns:   []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		sortByAscending: true,
		limit:           -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 1 {
		t.Error("Incorrect number of args returned")
	}
}
