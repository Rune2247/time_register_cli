package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func StartAssignment(d *db.DB, date, startTime, name string) error {
	if err := closeOpenEntry(d, date, startTime); err != nil {
		return err
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryAssignment,
		Name:      name,
		StartTime: startTime,
		EndTime:   "",
	}

	id, err := d.InsertEntry(entry)
	if err != nil {
		return fmt.Errorf("insert assignment: %w", err)
	}

	fmt.Printf("Started assignment %q at %s (id: %d)\n", name, startTime, id)
	TriggerSync(d)
	return nil
}
