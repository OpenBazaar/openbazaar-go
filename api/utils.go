package api

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"net/url"
	"strconv"
	"strings"
)

type TransactionQuery struct {
	OrderStates     []int    `json:"states"`
	SearchTerm      string   `json:"search"`
	SortByAscending bool     `json:"sortByAscending"`
	SortByRead      bool     `json:"sortByRead"`
	Limit           int      `json:"limit"`
	Exclude         []string `json:"exclude"`
}

func parseSearchTerms(q url.Values) (orderStates []pb.OrderState, searchTerm string, sortByAscending, sortByRead bool, limit int, err error) {
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "-1"
	}
	limit, err = strconv.Atoi(limitStr)
	if err != nil {
		return orderStates, searchTerm, false, false, 0, err
	}
	stateQuery := q.Get("state")
	states := strings.Split(stateQuery, ",")
	for _, s := range states {
		if s != "" {
			i, err := strconv.Atoi(s)
			if err != nil {
				return orderStates, searchTerm, false, false, 0, err
			}
			orderStates = append(orderStates, pb.OrderState(i))
		}
	}
	searchTerm = q.Get("search")
	sortTerms := strings.Split(q.Get("sortBy"), ",")
	if len(sortTerms) > 0 {
		for _, term := range sortTerms {
			switch strings.ToLower(term) {
			case "data-asc":
				sortByAscending = true
			case "read":
				sortByRead = true
			}
		}
	}
	return orderStates, searchTerm, sortByAscending, sortByRead, limit, nil
}

func convertOrderStates(states []int) []pb.OrderState {
	orderStates := make([]pb.OrderState, 0, len(states))
	for _, i := range states {
		orderStates = append(orderStates, pb.OrderState(i))
	}
	return orderStates
}
