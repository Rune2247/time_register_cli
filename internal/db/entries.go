package db

import (
	"database/sql"
	"fmt"

	"github.com/rlf/time_register_cli/internal/models"
)

func (d *DB) InsertEntry(e *models.Entry) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO entries (date, entry_type, name, start_time, end_time, duration_minutes, notes, posted_to_sheets, posted_to_calendar)
		 VALUES (?, ?, ?, ?, ?, ?, '', 0, 0)`,
		e.Date, e.EntryType, e.Name, e.StartTime, e.EndTime, e.DurationMinutes,
	)
	if err != nil {
		return 0, fmt.Errorf("insert entry: %w", err)
	}
	return res.LastInsertId()
}

func (d *DB) UpdateEntryEndTime(id int64, endTime string, durationMinutes int) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET end_time = ?, duration_minutes = ?, updated_at = datetime('now') WHERE id = ?`,
		endTime, durationMinutes, id,
	)
	return err
}

func (d *DB) GetEntriesByDate(date string) ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE date = ? ORDER BY start_time ASC`,
		date,
	)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (d *DB) GetEntriesByDateRange(startDate, endDate string) ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE date >= ? AND date <= ? ORDER BY date ASC, start_time ASC`,
		startDate, endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("query entries range: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (d *DB) GetLastOpenEntry(date string) (*models.Entry, error) {
	row := d.conn.QueryRow(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE date = ? AND end_time = '' ORDER BY start_time DESC LIMIT 1`,
		date,
	)
	return scanEntry(row)
}

// GetUnclosedDays returns dates (excluding today) where the last entry has no end_time.
func (d *DB) GetUnclosedDays(today string, limit int) ([]string, error) {
	rows, err := d.conn.Query(
		`SELECT DISTINCT e.date FROM entries e
		 WHERE e.date != ? AND e.end_time = '' AND e.start_time != ''
		 ORDER BY e.date DESC LIMIT ?`,
		today, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query unclosed days: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, fmt.Errorf("scan date: %w", err)
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

// GetDistinctDates returns all dates that have entries, most recent first.
func (d *DB) GetDistinctDates(limit int) ([]string, error) {
	rows, err := d.conn.Query(
		`SELECT DISTINCT date FROM entries ORDER BY date DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query distinct dates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, fmt.Errorf("scan date: %w", err)
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

// UpdateEntry updates an entry's name, start_time, end_time, and duration.
func (d *DB) UpdateEntry(id int64, name, startTime, endTime string, durationMinutes int) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET name = ?, start_time = ?, end_time = ?, duration_minutes = ?, updated_at = datetime('now') WHERE id = ?`,
		name, startTime, endTime, durationMinutes, id,
	)
	return err
}

// DeleteEntry deletes an entry by ID.
func (d *DB) DeleteEntry(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM entries WHERE id = ?`, id)
	return err
}

// DeleteEntriesByDateRange deletes all entries in a date range and returns the count.
func (d *DB) DeleteEntriesByDateRange(fromDate, toDate string) (int64, error) {
	res, err := d.conn.Exec(
		`DELETE FROM entries WHERE date >= ? AND date <= ?`, fromDate, toDate,
	)
	if err != nil {
		return 0, fmt.Errorf("delete entries in range: %w", err)
	}
	return res.RowsAffected()
}

// ResetSyncFlagsForDate marks all entries on a date as unsynced so they get re-posted.
func (d *DB) ResetSyncFlagsForDate(date string) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET posted_to_sheets = 0, posted_to_calendar = 0, updated_at = datetime('now') WHERE date = ?`,
		date,
	)
	return err
}

// ResetSyncFlagsForMonth marks all entries in a month (YYYY-MM) as unsynced.
func (d *DB) ResetSyncFlagsForMonth(yearMonth string) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET posted_to_sheets = 0, posted_to_calendar = 0, updated_at = datetime('now')
		 WHERE date LIKE ?`, yearMonth+"%",
	)
	return err
}

// GetUnsyncedForSheets returns entries not yet posted to sheets (any entry with a start time).
func (d *DB) GetUnsyncedForSheets() ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE posted_to_sheets = 0 AND start_time != ''
		 ORDER BY date ASC, start_time ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query unsynced sheets: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetUnsyncedForCalendar returns completed entries not yet posted to calendar (needs end_time).
func (d *DB) GetUnsyncedForCalendar() ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE posted_to_calendar = 0 AND end_time != ''
		 ORDER BY date ASC, start_time ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query unsynced calendar: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetUnsyncedEntries returns entries not yet fully synced (used by status display).
func (d *DB) GetUnsyncedEntries() ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE (posted_to_sheets = 0 AND start_time != '') OR (posted_to_calendar = 0 AND end_time != '')
		 ORDER BY date ASC, start_time ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query unsynced: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetAllEntries returns all entries with end_time set, ordered by date and start_time.
func (d *DB) GetAllEntries() ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes, notes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE end_time != '' ORDER BY date ASC, start_time ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query all entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// ResetAllCalendarFlags marks all entries as not posted to calendar.
func (d *DB) ResetAllCalendarFlags() error {
	_, err := d.conn.Exec(
		`UPDATE entries SET posted_to_calendar = 0, updated_at = datetime('now') WHERE end_time != ''`,
	)
	return err
}

// ResetAllSheetsFlags marks all entries as not posted to sheets.
func (d *DB) ResetAllSheetsFlags() error {
	_, err := d.conn.Exec(
		`UPDATE entries SET posted_to_sheets = 0, updated_at = datetime('now') WHERE start_time != ''`,
	)
	return err
}

func (d *DB) MarkPostedToSheets(id int64) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET posted_to_sheets = 1, updated_at = datetime('now') WHERE id = ?`, id,
	)
	return err
}

func (d *DB) MarkPostedToCalendar(id int64) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET posted_to_calendar = 1, updated_at = datetime('now') WHERE id = ?`, id,
	)
	return err
}

// AppendNote appends a timestamped note to the current open entry's notes field.
func (d *DB) AppendNote(id int64, note string) error {
	var current sql.NullString
	err := d.conn.QueryRow(`SELECT notes FROM entries WHERE id = ?`, id).Scan(&current)
	if err != nil {
		return fmt.Errorf("get notes: %w", err)
	}

	newNotes := note
	if current.String != "" {
		newNotes = current.String + "\n" + note
	}

	_, err = d.conn.Exec(
		`UPDATE entries SET notes = ?, posted_to_sheets = 0, updated_at = datetime('now') WHERE id = ?`,
		newNotes, id,
	)
	return err
}

// UpdateNotes replaces the notes field for an entry.
func (d *DB) UpdateNotes(id int64, notes string) error {
	_, err := d.conn.Exec(
		`UPDATE entries SET notes = ?, posted_to_sheets = 0, updated_at = datetime('now') WHERE id = ?`,
		notes, id,
	)
	return err
}

func scanEntries(rows *sql.Rows) ([]models.Entry, error) {
	var entries []models.Entry
	for rows.Next() {
		var e models.Entry
		var name, startTime, endTime, notes sql.NullString
		var duration sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.Date, &e.EntryType, &name, &startTime, &endTime, &duration, &notes,
			&e.PostedToSheets, &e.PostedToCalendar, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.Name = name.String
		e.StartTime = startTime.String
		e.EndTime = endTime.String
		e.Notes = notes.String
		if duration.Valid {
			e.DurationMinutes = int(duration.Int64)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func scanEntry(row *sql.Row) (*models.Entry, error) {
	var e models.Entry
	var name, startTime, endTime, notes sql.NullString
	var duration sql.NullInt64
	err := row.Scan(
		&e.ID, &e.Date, &e.EntryType, &name, &startTime, &endTime, &duration, &notes,
		&e.PostedToSheets, &e.PostedToCalendar, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan entry: %w", err)
	}
	e.Name = name.String
	e.StartTime = startTime.String
	e.EndTime = endTime.String
	e.Notes = notes.String
	if duration.Valid {
		e.DurationMinutes = int(duration.Int64)
	}
	return &e, nil
}
