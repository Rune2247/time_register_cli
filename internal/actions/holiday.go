package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func MarkHoliday(d *db.DB, date string) error {
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
	return nil
}
