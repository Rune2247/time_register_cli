package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func StartBreak(d *db.DB, date, startTime string) error {
	if err := closeOpenEntry(d, date, startTime); err != nil {
		return err
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryBreak,
		Name:      "Break",
		StartTime: startTime,
		EndTime:   "",
	}

	id, err := d.InsertEntry(entry)
	if err != nil {
		return fmt.Errorf("insert break: %w", err)
	}

	fmt.Printf("Started break at %s (id: %d)\n", startTime, id)
	TriggerSync(d)
	return nil
}
