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

	dayStatus, err := GetDayStatus(d, date)
	if err != nil {
		return err
	}
	weekStatus, err := GetWeekStatus(d, date)
	if err != nil {
		return err
	}

	fmt.Printf("Day ended at %s\n", endTime)
	fmt.Println(models.FormatStatusSummary(dayStatus, weekStatus))

	return nil
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
