package biz

import (
	"fmt"
	"time"
)

// ISOWeekOf returns the ISO-8601 week label for t in the form "YYYY-Www"
// (e.g. "2026-W23"). The year is the ISO week-numbering year (which may differ
// from the calendar year at year boundaries), per time.ISOWeek.
func ISOWeekOf(t time.Time) string {
	year, week := t.UTC().ISOWeek()

	return fmt.Sprintf("%04d-W%02d", year, week)
}
