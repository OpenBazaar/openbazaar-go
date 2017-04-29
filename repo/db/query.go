package db

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"strconv"
	"strings"
)

type query struct {
	table         string
	columns       []string
	offsetId      string
	stateFilter   []pb.OrderState
	searchTerm    string
	searchColumns []string
	ascending     bool
	limit         int
}

func filterQuery(q query) (stm string, args []interface{}) {

	stateFilterClause := ""
	var states []int
	if len(q.stateFilter) > 0 {
		stateFilterClauseParts := make([]string, 0, len(q.stateFilter))

		for i := 0; i < len(q.stateFilter); i++ {
			states = append(states, int(q.stateFilter[i]))
			stateFilterClauseParts = append(stateFilterClauseParts, "?")
		}

		stateFilterClause = "state in (" + strings.Join(stateFilterClauseParts, ",") + ")"
	}
	order := "desc"
	operator := "<"
	if q.ascending {
		order = "asc"
		operator = ">"
	}

	var filter string
	var search string

	searchFilter := `(`
	for i, c := range q.searchColumns {
		searchFilter += c
		if i < len(q.searchColumns)-1 {
			searchFilter += " || "
		}
	}
	searchFilter += `)`

	queryColumns := ``
	for i, c := range q.columns {
		queryColumns += c
		if i < len(q.columns)-1 {
			queryColumns += ", "
		}
	}

	searchTableSortBy := "orderID"
	if q.table == "cases" {
		searchTableSortBy = "caseID"
	}

	if q.offsetId != "" {
		args = append(args, q.offsetId)
		if stateFilterClause != "" {
			filter = " and " + stateFilterClause
		}
		if q.searchTerm != "" {
			search = " and " + searchFilter + " like ?"
		}
		stm = "select " + queryColumns + " from " + q.table + " where timestamp" + operator + "(select timestamp from " + q.table + " where " + searchTableSortBy + "=?)" + filter + search + " order by timestamp " + order + " limit " + strconv.Itoa(q.limit) + ";"
	} else {
		if stateFilterClause != "" {
			filter = " where " + stateFilterClause
		}
		if q.searchTerm != "" {
			if filter == "" {
				search = " where " + searchFilter + " like ?"
			} else {
				search = " and " + searchFilter + " like ?"
			}
		}
		stm = "select " + queryColumns + " from " + q.table + filter + search + " order by timestamp " + order + " limit " + strconv.Itoa(q.limit) + ";"
	}
	for _, s := range states {
		args = append(args, s)
	}
	if q.searchTerm != "" {
		args = append(args, "%"+q.searchTerm+"%")
	}
	return stm, args
}
