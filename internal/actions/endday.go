package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func EndDay(d *db.DB, date, endTime string) error {
	if err := endDay(d, date, endTime); err != nil {
		return err
	}

	// Print day and week status
	dayStatus, err := GetDayStatus(d, date)
	if err != nil {
		return err
	}
	weekStatus, err := GetWeekStatus(d, date)
	if err != nil {
		return err
	}

	fmt.Printf("Day ended at %s\n", endTime)
	fmt.Printf("Today you have worked %.1f hours\n", float64(dayStatus.WorkMinutes)/60.0)
	fmt.Printf("This week you have worked %.1f hours, and had %.1f lunch hours\n",
		float64(weekStatus.WorkMinutes)/60.0, float64(weekStatus.LunchMinutes)/60.0)

	return nil
}

// EndDaySilent ends the day without printing to stdout (for background worker).
func EndDaySilent(d *db.DB, date, endTime string) error {
	return endDay(d, date, endTime)
}

func endDay(d *db.DB, date, endTime string) error {
	if err := closeOpenEntry(d, date, endTime); err != nil {
		return err
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryEndDay,
		StartTime: endTime,
		EndTime:   endTime,
	}

	if _, err := d.InsertEntry(entry); err != nil {
		return fmt.Errorf("insert end_day: %w", err)
	}

	return nil
}
