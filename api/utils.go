package api

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"net/url"
	"strconv"
	"strings"
)

func parseSearchTerms(q url.Values) (searchTerm, orderStates []pb.OrderState, sortByAscending, sortByRead bool, limit int, exclude []string, err error) {
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "-1"
	}
	limit, err = strconv.Atoi(limitStr)
	if err != nil {
		return "", "", []pb.OrderState{}, false, false, 0, err
	}
	stateQuery := q.Get("state")
	states := strings.Split(stateQuery, ",")
	for _, s := range states {
		if s != "" {
			i, err := strconv.Atoi(s)
			if err != nil {
				return "", "", []pb.OrderState{}, false, false, 0, err
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
	excludeQuery := q.Get("exclude")
	exclude = strings.Split(excludeQuery, ",")
	return searchTerm, orderStates, sortByAscending, sortByRead, limit, exclude, nil
}
