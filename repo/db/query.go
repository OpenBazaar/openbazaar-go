package db

import (
	"strconv"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

type query struct {
	table           string
	columns         []string
	stateFilter     []pb.OrderState
	searchTerm      string
	searchColumns   []string
	sortByAscending bool
	sortByRead      bool
	id              string
	exclude         []string
	limit           int
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
	var exclude string
	if len(q.exclude) > 0 {
		excludeFilterClauseParts := make([]string, 0, len(q.exclude))
		for i := 0; i < len(q.exclude); i++ {
			excludeFilterClauseParts = append(excludeFilterClauseParts, "?")
		}
		exclude = q.id + " not in (" + strings.Join(excludeFilterClauseParts, ",") + ")"
	}

	order := "desc"
	if q.sortByAscending {
		order = "asc"
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

	var readSort string
	if q.sortByRead {
		readSort = "read asc, "
	}

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
	if exclude != "" {
		if filter != "" || search != "" {
			exclude = " and " + exclude
		} else {
			exclude = " where " + exclude
		}
	}
	stm = "select " + queryColumns + " from " + q.table + filter + search + exclude + " order by " + readSort + "timestamp " + order + " limit " + strconv.Itoa(q.limit) + ";"

	for _, s := range states {
		args = append(args, s)
	}
	if q.searchTerm != "" {
		args = append(args, "%"+q.searchTerm+"%")
	}
	if len(q.exclude) > 0 {
		for _, s := range q.exclude {
			args = append(args, s)
		}
	}
	return stm, args
}
