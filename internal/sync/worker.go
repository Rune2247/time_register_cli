package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rlf/time_register_cli/internal/actions"
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
	ctx := context.Background()
	spreadsheetID, calendarID := actions.GetGoogleConfig(w.db)

	var sheetsClient *googleapi.SheetsClient
	var calendarClient *googleapi.CalendarClient
	var err error

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

	// Sync sheets — any entry with a start time
	if sheetsClient != nil {
		sheetEntries, err := w.db.GetUnsyncedForSheets()
		if err != nil {
			return fmt.Errorf("get unsynced for sheets: %w", err)
		}
		for _, entry := range sheetEntries {
			w.syncToSheets(entry, sheetsClient)
		}
	}

	// Sync calendar — only complete entries with end_time
	if calendarClient != nil {
		calEntries, err := w.db.GetUnsyncedForCalendar()
		if err != nil {
			return fmt.Errorf("get unsynced for calendar: %w", err)
		}
		for _, entry := range calEntries {
			w.syncToCalendar(entry, calendarClient)
		}
	}

	return nil
}

func (w *Worker) syncToSheets(entry models.Entry, client *googleapi.SheetsClient) {
	if err := client.WriteEntry(&entry); err != nil {
		log.Printf("sheets sync failed for entry %d: %v", entry.ID, err)
	} else {
		if err := w.db.MarkPostedToSheets(entry.ID); err != nil {
			log.Printf("mark posted to sheets failed for entry %d: %v", entry.ID, err)
		}
	}
}

func (w *Worker) syncToCalendar(entry models.Entry, client *googleapi.CalendarClient) {
	if err := client.CreateEvent(&entry); err != nil {
		log.Printf("calendar sync failed for entry %d: %v", entry.ID, err)
	} else {
		if err := w.db.MarkPostedToCalendar(entry.ID); err != nil {
			log.Printf("mark posted to calendar failed for entry %d: %v", entry.ID, err)
		}
	}
}
