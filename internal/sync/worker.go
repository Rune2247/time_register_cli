package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/rlf/time_register_cli/internal/models"
)

type Worker struct {
	db       *db.DB
	interval time.Duration
	stop     chan struct{}
}

func NewWorker(d *db.DB, interval time.Duration) *Worker {
	return &Worker{
		db:       d,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins the background sync loop.
func (w *Worker) Start() {
	go w.run()
}

// Stop signals the worker to stop.
func (w *Worker) Stop() {
	close(w.stop)
}

// SyncNow runs a single sync cycle. Can be called directly for immediate sync.
func (w *Worker) SyncNow() error {
	return w.syncOnce()
}

func (w *Worker) run() {
	// Run once immediately
	if err := w.syncOnce(); err != nil {
		log.Printf("sync error: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.syncOnce(); err != nil {
				log.Printf("sync error: %v", err)
			}
		case <-w.stop:
			return
		}
	}
}

func (w *Worker) syncOnce() error {
	entries, err := w.db.GetUnsyncedEntries()
	if err != nil {
		return fmt.Errorf("get unsynced: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	ctx := context.Background()

	// Get config
	spreadsheetID, _ := w.db.GetConfig("spreadsheet_id")
	calendarID, _ := w.db.GetConfig("calendar_id")

	var sheetsClient *googleapi.SheetsClient
	var calendarClient *googleapi.CalendarClient

	if spreadsheetID != "" {
		sheetsClient, err = googleapi.NewSheetsClient(ctx, spreadsheetID)
		if err != nil {
			log.Printf("sheets client error (will retry): %v", err)
		}
	}

	if calendarID != "" {
		calendarClient, err = googleapi.NewCalendarClient(ctx, calendarID)
		if err != nil {
			log.Printf("calendar client error (will retry): %v", err)
		}
	}

	for _, entry := range entries {
		w.syncEntry(entry, sheetsClient, calendarClient)
	}

	return nil
}

func (w *Worker) syncEntry(entry models.Entry, sheetsClient *googleapi.SheetsClient, calendarClient *googleapi.CalendarClient) {
	// Sync to Sheets
	if !entry.PostedToSheets && sheetsClient != nil {
		if err := sheetsClient.WriteEntry(&entry); err != nil {
			log.Printf("sheets sync failed for entry %d: %v", entry.ID, err)
		} else {
			if err := w.db.MarkPostedToSheets(entry.ID); err != nil {
				log.Printf("mark posted to sheets failed for entry %d: %v", entry.ID, err)
			}
		}
	}

	// Sync to Calendar
	if !entry.PostedToCalendar && calendarClient != nil {
		// Skip end_day entries — they're just markers
		if entry.EntryType == models.EntryEndDay {
			if err := w.db.MarkPostedToCalendar(entry.ID); err != nil {
				log.Printf("mark posted to calendar failed for entry %d: %v", entry.ID, err)
			}
			return
		}

		if err := calendarClient.CreateEvent(&entry); err != nil {
			log.Printf("calendar sync failed for entry %d: %v", entry.ID, err)
		} else {
			if err := w.db.MarkPostedToCalendar(entry.ID); err != nil {
				log.Printf("mark posted to calendar failed for entry %d: %v", entry.ID, err)
			}
		}
	}
}
