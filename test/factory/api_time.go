package factory

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewAPITime(t time.Time) *repo.APITime {
	return repo.NewAPITime(t)
}
