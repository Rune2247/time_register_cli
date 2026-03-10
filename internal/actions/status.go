package actions

import (
	"fmt"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func GetDayStatus(d *db.DB, date string) (*models.DayStatus, error) {
	entries, err := d.GetEntriesByDate(date)
	if err != nil {
		return nil, err
	}

	work, lunch, brk := models.AccumulateMinutes(entries)
	return &models.DayStatus{
		Date:         date,
		Entries:      entries,
		WorkMinutes:  work,
		LunchMinutes: lunch,
		BreakMinutes: brk,
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
	return &models.WeekStatus{
		StartDate:    startDate,
		EndDate:      endDate,
		WorkMinutes:  work,
		LunchMinutes: lunch,
		BreakMinutes: brk,
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
	}

	return nil
}
