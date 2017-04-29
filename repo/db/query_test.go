package db

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func Test_filterQuery(t *testing.T) {
	stm, args := filterQuery(query{
		table:         "purchases",
		columns:       []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offsetId:      "asdf",
		stateFilter:   []pb.OrderState{},
		searchTerm:    "test",
		searchColumns: []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		ascending:     false,
		limit:         -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where timestamp<(select timestamp from purchases where orderID=?) and (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp desc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 2 {
		t.Error("Incorrect number of args returned")
	}

	stm, args = filterQuery(query{
		table:         "purchases",
		columns:       []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offsetId:      "asdf",
		stateFilter:   []pb.OrderState{pb.OrderState_PENDING},
		searchTerm:    "test",
		searchColumns: []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		ascending:     true,
		limit:         -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where timestamp>(select timestamp from purchases where orderID=?) and state in (?) and (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 3 {
		t.Error("Incorrect number of args returned")
	}

	stm, args = filterQuery(query{
		table:         "purchases",
		columns:       []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offsetId:      "",
		stateFilter:   []pb.OrderState{pb.OrderState_PENDING},
		searchTerm:    "test",
		searchColumns: []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		ascending:     true,
		limit:         -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where state in (?) and (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 2 {
		t.Error("Incorrect number of args returned")
	}

	stm, args = filterQuery(query{
		table:         "purchases",
		columns:       []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offsetId:      "",
		stateFilter:   []pb.OrderState{pb.OrderState_PENDING},
		searchTerm:    "",
		searchColumns: []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		ascending:     true,
		limit:         -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where state in (?) order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 1 {
		t.Error("Incorrect number of args returned")
	}

	stm, args = filterQuery(query{
		table:         "purchases",
		columns:       []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "state", "read"},
		offsetId:      "",
		stateFilter:   []pb.OrderState{},
		searchTerm:    "asdf",
		searchColumns: []string{"orderID", "timestamp", "total", "title", "thumbnail", "vendorID", "vendorBlockchainID", "shippingName", "shippingAddress", "paymentAddr"},
		ascending:     true,
		limit:         -1,
	})
	if stm != `select orderID, timestamp, total, title, thumbnail, vendorID, vendorBlockchainID, shippingName, shippingAddress, state, read from purchases where (orderID || timestamp || total || title || thumbnail || vendorID || vendorBlockchainID || shippingName || shippingAddress || paymentAddr) like ? order by timestamp asc limit -1;` {
		t.Error("Returned invalid query string")
	}
	if len(args) != 1 {
		t.Error("Incorrect number of args returned")
	}
}
