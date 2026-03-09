package actions

import (
	"fmt"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
)

// closeOpenEntry closes the last open entry on the given date by setting its
// end_time and calculating duration. Called before starting a new entry.
func closeOpenEntry(d *db.DB, date, endTime string) error {
	open, err := d.GetLastOpenEntry(date)
	if err != nil {
		return fmt.Errorf("get open entry: %w", err)
	}
	if open == nil {
		return nil
	}

	duration, err := calcDurationMinutes(open.StartTime, endTime)
	if err != nil {
		return fmt.Errorf("calc duration: %w", err)
	}

	return d.UpdateEntryEndTime(open.ID, endTime, duration)
}

func calcDurationMinutes(startTime, endTime string) (int, error) {
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return 0, fmt.Errorf("parse start time %q: %w", startTime, err)
	}
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return 0, fmt.Errorf("parse end time %q: %w", endTime, err)
	}
	return int(end.Sub(start).Minutes()), nil
}

// NowInCopenhagen returns the current time in Europe/Copenhagen as HH:MM.
func NowInCopenhagen() string {
	loc, _ := time.LoadLocation("Europe/Copenhagen")
	return time.Now().In(loc).Format("15:04")
}

// TodayInCopenhagen returns today's date in Europe/Copenhagen as YYYY-MM-DD.
func TodayInCopenhagen() string {
	loc, _ := time.LoadLocation("Europe/Copenhagen")
	return time.Now().In(loc).Format("2006-01-02")
}
