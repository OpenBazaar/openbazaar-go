package core

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type disputeNotifier struct {
	disputeCasesDb repo.CaseStore
}

func (d *disputeNotifier) PerformTask() (err error) {
	disputes, err := d.disputeCasesDb.GetDisputesForNotification()
	if err != nil {
		return
	}

	executedAt := time.Now()
	for _, d := range disputes {
		d.LastNotifiedAt = executedAt.Unix()
	}

	err = d.disputeCasesDb.UpdateDisputesLastNotifiedAt(disputes)
	return
}
