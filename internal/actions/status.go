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

	status := &models.DayStatus{
		Date:    date,
		Entries: entries,
	}

	for _, e := range entries {
		if e.DurationMinutes <= 0 {
			continue
		}
		switch e.EntryType {
		case models.EntryLunch:
			status.LunchMinutes += e.DurationMinutes
		case models.EntryAssignment:
			status.WorkMinutes += e.DurationMinutes
		}
	}

	return status, nil
}

func GetWeekStatus(d *db.DB, date string) (*models.WeekStatus, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("parse date: %w", err)
	}

	// Find Monday of this week
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

	status := &models.WeekStatus{
		StartDate: startDate,
		EndDate:   endDate,
	}

	for _, e := range entries {
		if e.DurationMinutes <= 0 {
			continue
		}
		switch e.EntryType {
		case models.EntryLunch:
			status.LunchMinutes += e.DurationMinutes
		case models.EntryAssignment:
			status.WorkMinutes += e.DurationMinutes
		}
	}

	return status, nil
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

	fmt.Printf("Today you have worked %.1f hours\n", float64(dayStatus.WorkMinutes)/60.0)
	fmt.Printf("This week you have worked %.1f hours, and had %.1f lunch hours\n",
		float64(weekStatus.WorkMinutes)/60.0, float64(weekStatus.LunchMinutes)/60.0)

	// Print today's entries
	if len(dayStatus.Entries) > 0 {
		fmt.Println("\nToday's entries:")
		for _, e := range dayStatus.Entries {
			switch e.EntryType {
			case models.EntryAssignment:
				end := e.EndTime
				if end == "" {
					end = "ongoing"
				}
				fmt.Printf("  %s - %s  %s\n", e.StartTime, end, e.Name)
			case models.EntryLunch:
				end := e.EndTime
				if end == "" {
					end = "ongoing"
				}
				fmt.Printf("  %s - %s  Lunch\n", e.StartTime, end)
			case models.EntryHoliday:
				fmt.Printf("  Holiday\n")
			case models.EntryEndDay:
				fmt.Printf("  %s        Day ended\n", e.StartTime)
			}
		}
	}

	return nil
}
