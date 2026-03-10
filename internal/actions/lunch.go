package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func StartLunch(d *db.DB, date, triggerTime string) error {
	if err := closeOpenEntry(d, date, triggerTime); err != nil {
		return err
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryLunch,
		Name:      "Lunch",
		StartTime: triggerTime,
		EndTime:   "",
	}

	id, err := d.InsertEntry(entry)
	if err != nil {
		return fmt.Errorf("insert lunch: %w", err)
	}

	fmt.Printf("Lunch started at %s (id: %d)\n", triggerTime, id)
	return nil
}
