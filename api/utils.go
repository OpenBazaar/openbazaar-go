package api

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"net/url"
	"strconv"
	"strings"
)

func parseSearchTerms(q url.Values) (searchTerm, offsetID string, orderStates []pb.OrderState, ascending bool, limit int, err error) {
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "-1"
	}
	limit, err = strconv.Atoi(limitStr)
	if err != nil {
		return "", "", []pb.OrderState{}, false, 0, err
	}
	offsetID = q.Get("offsetId")
	stateQuery := q.Get("state")
	states := strings.Split(stateQuery, ",")
	for _, s := range states {
		if s != "" {
			i, err := strconv.Atoi(s)
			if err != nil {
				return "", "", []pb.OrderState{}, false, 0, err
			}
			orderStates = append(orderStates, pb.OrderState(i))
		}
	}
	searchTerm = q.Get("search")
	if q.Get("sortBy") == "date-asc" {
		ascending = true
	}
	return searchTerm, offsetID, orderStates, ascending, limit, nil
}
