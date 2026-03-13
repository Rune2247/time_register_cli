package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

// openEntryElapsed calculates elapsed minutes for any open entries using the current time.
func openEntryElapsed(entries []models.Entry) (work, lunch, brk int) {
	now := NowInCopenhagen()
	for _, e := range entries {
		if e.EndTime == "" && e.StartTime != "" {
			elapsed, err := CalcDurationMinutes(e.StartTime, now)
			if err != nil || elapsed <= 0 {
				continue
			}
			switch e.EntryType {
			case models.EntryAssignment:
				work += elapsed
			case models.EntryLunch:
				lunch += elapsed
			case models.EntryBreak:
				brk += elapsed
			}
		}
	}
	return
}

func GetDayStatus(d *db.DB, date string) (*models.DayStatus, error) {
	entries, err := d.GetEntriesByDate(date)
	if err != nil {
		return nil, err
	}

	work, lunch, brk := models.AccumulateMinutes(entries)
	ow, ol, ob := openEntryElapsed(entries)
	return &models.DayStatus{
		Date:         date,
		Entries:      entries,
		WorkMinutes:  work + ow,
		LunchMinutes: lunch + ol,
		BreakMinutes: brk + ob,
	}, nil
}

func GetWeekStatus(d *db.DB, date string) (*models.WeekStatus, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("parse date: %w", err)
	}

	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := t.AddDate(0, 0, -int(weekday-time.Monday))
	sunday := monday.AddDate(0, 0, 6)

	startDate := monday.Format("2006-01-02")
	endDate := sunday.Format("2006-01-02")

	entries, err := d.GetEntriesByDateRange(startDate, endDate)
	if err != nil {
		return nil, err
	}

	work, lunch, brk := models.AccumulateMinutes(entries)
	ow, ol, ob := openEntryElapsed(entries)
	return &models.WeekStatus{
		StartDate:    startDate,
		EndDate:      endDate,
		WorkMinutes:  work + ow,
		LunchMinutes: lunch + ol,
		BreakMinutes: brk + ob,
	}, nil
}

func PrintStatus(d *db.DB, date string) error {
	dayStatus, err := GetDayStatus(d, date)
	if err != nil {
		return err
	}
	weekStatus, err := GetWeekStatus(d, date)
	if err != nil {
		return err
	}

	fmt.Println(models.FormatStatusSummary(dayStatus, weekStatus))

	if len(dayStatus.Entries) > 0 {
		fmt.Println("\nToday's entries:")
		for i := range dayStatus.Entries {
			fmt.Printf("  %s\n", dayStatus.Entries[i].FormatLine())
		}

		// Show notes for the current open entry
		for i := range dayStatus.Entries {
			e := &dayStatus.Entries[i]
			if e.EndTime == "" && e.Notes != "" {
				fmt.Printf("\nNotes (%s):\n", e.DisplayName())
				for _, line := range strings.Split(e.Notes, "\n") {
					fmt.Printf("  %s\n", line)
				}
			}
		}
	}

	return nil
}
