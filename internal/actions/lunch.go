package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func StartLunch(d *db.DB, date, triggerTime string) error {
	defaultLunch, err := d.GetConfig("default_lunch_time")
	if err != nil {
		return err
	}
	if defaultLunch == "" {
		defaultLunch = "12:00"
	}

	// If triggered after default lunch time, adjust: lunch starts at default time
	lunchStart := triggerTime
	if triggerTime > defaultLunch {
		lunchStart = defaultLunch
	}

	// Close the previous entry at the lunch start time
	if err := closeOpenEntry(d, date, lunchStart); err != nil {
		return err
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryLunch,
		Name:      "Lunch",
		StartTime: lunchStart,
		EndTime:   "",
	}

	id, err := d.InsertEntry(entry)
	if err != nil {
		return fmt.Errorf("insert lunch: %w", err)
	}

	fmt.Printf("Lunch started at %s (id: %d)\n", lunchStart, id)
	return nil
}
