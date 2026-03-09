package db

import (
	"database/sql"
	"fmt"

	"github.com/rlf/time_register_cli/internal/models"
)

func (d *DB) InsertEntry(e *models.Entry) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO entries (date, entry_type, name, start_time, end_time, duration_minutes, posted_to_sheets, posted_to_calendar)
		 VALUES (?, ?, ?, ?, ?, ?, 0, 0)`,
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
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes,
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
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes,
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
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE date = ? AND end_time = '' ORDER BY start_time DESC LIMIT 1`,
		date,
	)
	return scanEntry(row)
}

// RemoveEndDay deletes the end_day entry for a date and reopens the last
// assignment (clears its end_time). Returns true if an entry was removed.
func (d *DB) RemoveEndDay(date string) (bool, error) {
	res, err := d.conn.Exec(
		`DELETE FROM entries WHERE date = ? AND entry_type = 'end_day'`, date,
	)
	if err != nil {
		return false, fmt.Errorf("delete end_day: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return false, nil
	}

	// Reopen the last assignment on that date (clear its end_time)
	_, err = d.conn.Exec(
		`UPDATE entries SET end_time = '', duration_minutes = 0, updated_at = datetime('now')
		 WHERE id = (
			SELECT id FROM entries
			WHERE date = ? AND entry_type IN ('assignment', 'lunch')
			ORDER BY start_time DESC LIMIT 1
		 )`, date,
	)
	if err != nil {
		return true, fmt.Errorf("reopen last entry: %w", err)
	}

	return true, nil
}

func (d *DB) GetUnsyncedEntries() ([]models.Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, date, entry_type, name, start_time, end_time, duration_minutes,
		        posted_to_sheets, posted_to_calendar, created_at, updated_at
		 FROM entries WHERE (posted_to_sheets = 0 OR posted_to_calendar = 0) AND end_time != ''
		 ORDER BY date ASC, start_time ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query unsynced: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
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

func scanEntries(rows *sql.Rows) ([]models.Entry, error) {
	var entries []models.Entry
	for rows.Next() {
		var e models.Entry
		var name, startTime, endTime sql.NullString
		var duration sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.Date, &e.EntryType, &name, &startTime, &endTime, &duration,
			&e.PostedToSheets, &e.PostedToCalendar, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.Name = name.String
		e.StartTime = startTime.String
		e.EndTime = endTime.String
		if duration.Valid {
			e.DurationMinutes = int(duration.Int64)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func scanEntry(row *sql.Row) (*models.Entry, error) {
	var e models.Entry
	var name, startTime, endTime sql.NullString
	var duration sql.NullInt64
	err := row.Scan(
		&e.ID, &e.Date, &e.EntryType, &name, &startTime, &endTime, &duration,
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
	if duration.Valid {
		e.DurationMinutes = int(duration.Int64)
	}
	return &e, nil
}
