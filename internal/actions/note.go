package actions

import (
	"fmt"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	"github.com/rlf/time_register_cli/internal/models"
)

func AddNote(d *db.DB, text string) error {
	today := TodayInCopenhagen()
	open, err := d.GetLastOpenEntry(today)
	if err != nil {
		return fmt.Errorf("get open entry: %w", err)
	}
	if open == nil {
		fmt.Println("No active entry — note not saved.")
		return nil
	}

	now := time.Now().In(models.CopenhagenTZ)
	timestamp := fmt.Sprintf("%02d:%02d %d/%d", now.Hour(), now.Minute(), now.Day(), int(now.Month()))
	note := fmt.Sprintf("%s: %s", timestamp, text)

	if err := d.AppendNote(open.ID, note); err != nil {
		return fmt.Errorf("append note: %w", err)
	}

	fmt.Printf("Note added to %s: %s\n", open.DisplayName(), note)
	TriggerSync(d)
	return nil
}
