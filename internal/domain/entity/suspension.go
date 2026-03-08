package entity

import (
	"time"
)

type SuspensionDuration string

const (
	SuspensionDuration7Days     SuspensionDuration = "7d"
	SuspensionDuration15Days    SuspensionDuration = "15d"
	SuspensionDuration30Days    SuspensionDuration = "30d"
	SuspensionDurationUnlimited SuspensionDuration = "unlimited"
)

func (d SuspensionDuration) EndTime(now time.Time) (*time.Time, bool) {
	switch d {
	case SuspensionDuration7Days:
		until := now.AddDate(0, 0, 7)
		return &until, true
	case SuspensionDuration15Days:
		until := now.AddDate(0, 0, 15)
		return &until, true
	case SuspensionDuration30Days:
		until := now.AddDate(0, 0, 30)
		return &until, true
	case SuspensionDurationUnlimited:
		return nil, true
	default:
		return nil, false
	}
}

func ParseSuspensionDuration(raw string) (SuspensionDuration, bool) {
	switch SuspensionDuration(raw) {
	case SuspensionDuration7Days, SuspensionDuration15Days, SuspensionDuration30Days, SuspensionDurationUnlimited:
		return SuspensionDuration(raw), true
	default:
		return "", false
	}
}
