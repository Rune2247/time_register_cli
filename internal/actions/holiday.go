package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func MarkHoliday(d *db.DB, date string) error {
	// Delete any existing entries for the day
	existing, err := d.GetEntriesByDate(date)
	if err != nil {
		return fmt.Errorf("get entries: %w", err)
	}
	for _, e := range existing {
		if err := d.DeleteEntry(e.ID); err != nil {
			return fmt.Errorf("delete entry %d: %w", e.ID, err)
		}
	}
	if len(existing) > 0 {
		fmt.Printf("Cleared %d existing entries for %s\n", len(existing), date)
	}

	entry := &models.Entry{
		Date:      date,
		EntryType: models.EntryHoliday,
		Name:      "Holiday",
	}

	id, err := d.InsertEntry(entry)
	if err != nil {
		return fmt.Errorf("insert holiday: %w", err)
	}

	fmt.Printf("Marked %s as holiday (id: %d)\n", date, id)
	TriggerSync(d)
	return nil
}
