package dline

import "github.com/filecoin-project/go-state-types/abi"

// Deadline calculations with respect to a current epoch.
// "Deadline" refers to the window during which proofs may be submitted.
// Windows are non-overlapping ranges [Open, Close), but the challenge epoch for a window occurs before
// the window opens.
// The current epoch may not necessarily lie within the deadline or proving period represented here.
type Info struct {
	// Deadline parameters
	CurrentEpoch abi.ChainEpoch // Epoch at which this info was calculated.
	PeriodStart  abi.ChainEpoch // First epoch of the proving period (<= CurrentEpoch).
	Index        uint64         // A deadline index, in [0..d.WPoStProvingPeriodDeadlines) unless period elapsed.
	Open         abi.ChainEpoch // First epoch from which a proof may be submitted (>= CurrentEpoch).
	Close        abi.ChainEpoch // First epoch from which a proof may no longer be submitted (>= Open).
	Challenge    abi.ChainEpoch // Epoch at which to sample the chain for challenge (< Open).
	FaultCutoff  abi.ChainEpoch // First epoch at which a fault declaration is rejected (< Open).

	// Protocol parameters
	WPoStPeriodDeadlines   uint64
	WPoStProvingPeriod     abi.ChainEpoch // the number of epochs in a window post proving period
	WPoStChallengeWindow   abi.ChainEpoch
	WPoStChallengeLookback abi.ChainEpoch
	FaultDeclarationCutoff abi.ChainEpoch
}

// Whether the proving period has begun.
func (d *Info) PeriodStarted() bool {
	return d.CurrentEpoch >= d.PeriodStart
}

// Whether the proving period has elapsed.
func (d *Info) PeriodElapsed() bool {
	return d.CurrentEpoch >= d.NextPeriodStart()
}

// The last epoch in the proving period.
func (d *Info) PeriodEnd() abi.ChainEpoch {
	return d.PeriodStart + d.WPoStProvingPeriod - 1
}

// The first epoch in the next proving period.
func (d *Info) NextPeriodStart() abi.ChainEpoch {
	return d.PeriodStart + d.WPoStProvingPeriod
}

// Whether the current deadline is currently open.
func (d *Info) IsOpen() bool {
	return d.CurrentEpoch >= d.Open && d.CurrentEpoch < d.Close
}

// Whether the current deadline has already closed.
func (d *Info) HasElapsed() bool {
	return d.CurrentEpoch >= d.Close
}

// The last epoch during which a proof may be submitted.
func (d *Info) Last() abi.ChainEpoch {
	return d.Close - 1
}

// Epoch at which the subsequent deadline opens.
func (d *Info) NextOpen() abi.ChainEpoch {
	return d.Close
}

// Whether the deadline's fault cutoff has passed.
func (d *Info) FaultCutoffPassed() bool {
	return d.CurrentEpoch >= d.FaultCutoff
}

// Returns the next instance of this deadline that has not yet elapsed.
func (d *Info) NextNotElapsed() *Info {
	next := d
	for next.HasElapsed() {
		next = NewInfo(next.NextPeriodStart(), d.Index, d.CurrentEpoch, d.WPoStPeriodDeadlines, d.WPoStProvingPeriod, d.WPoStChallengeWindow, d.WPoStChallengeLookback, d.FaultDeclarationCutoff)
	}
	return next
}

// Returns deadline-related calculations for a deadline in some proving period and the current epoch.
func NewInfo(periodStart abi.ChainEpoch, deadlineIdx uint64, currEpoch abi.ChainEpoch, wPoStPeriodDeadlines uint64, wPoStProvingPeriod, wPoStChallengeWindow, wPoStChallengeLookback, faultDeclarationCutoff abi.ChainEpoch) *Info {
	if deadlineIdx < wPoStPeriodDeadlines {
		deadlineOpen := periodStart + (abi.ChainEpoch(deadlineIdx) * wPoStChallengeWindow)
		return &Info{
			CurrentEpoch: currEpoch,
			PeriodStart:  periodStart,
			Index:        deadlineIdx,
			Open:         deadlineOpen,
			Close:        deadlineOpen + wPoStChallengeWindow,
			Challenge:    deadlineOpen - wPoStChallengeLookback,
			FaultCutoff:  deadlineOpen - faultDeclarationCutoff,
			// parameters
			WPoStPeriodDeadlines:   wPoStPeriodDeadlines,
			WPoStProvingPeriod:     wPoStProvingPeriod,
			WPoStChallengeWindow:   wPoStChallengeWindow,
			WPoStChallengeLookback: wPoStChallengeLookback,
			FaultDeclarationCutoff: faultDeclarationCutoff,
		}
	} else {
		// Return deadline info for a no-duration deadline immediately after the last real one.
		afterLastDeadline := periodStart + wPoStProvingPeriod
		return &Info{
			CurrentEpoch: currEpoch,
			PeriodStart:  periodStart,
			Index:        deadlineIdx,
			Open:         afterLastDeadline,
			Close:        afterLastDeadline,
			Challenge:    afterLastDeadline,
			FaultCutoff:  0,
			// parameters
			WPoStPeriodDeadlines:   wPoStPeriodDeadlines,
			WPoStProvingPeriod:     wPoStProvingPeriod,
			WPoStChallengeWindow:   wPoStChallengeWindow,
			WPoStChallengeLookback: wPoStChallengeLookback,
			FaultDeclarationCutoff: faultDeclarationCutoff,
		}
	}
}
