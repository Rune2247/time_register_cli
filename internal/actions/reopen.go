package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
)

// ReopenDay removes the end_day entry for a given date, allowing
// more assignments to be added. If there's no end_day entry, it's a no-op.
func ReopenDay(d *db.DB, date string) error {
	removed, err := d.RemoveEndDay(date)
	if err != nil {
		return fmt.Errorf("reopen day: %w", err)
	}
	if !removed {
		fmt.Printf("No end-of-day entry found for %s\n", date)
		return nil
	}
	fmt.Printf("Reopened %s — you can now add more entries\n", date)
	return nil
}
