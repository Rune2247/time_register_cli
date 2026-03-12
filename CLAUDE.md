# TimeReg CLI

Go-based CLI tool for time registration that syncs to Google Sheets and Google Calendar.

## Project Structure

```
cmd/timereg/main.go          # Entry point
internal/
  actions/                   # Business logic (start, lunch, break, end, edit, etc.)
  cli/                       # Cobra commands, TUI menu, daemon, setup guide
  db/                        # SQLite persistence (entries + config tables)
  google/                    # OAuth2, Sheets API, Calendar API
  models/                    # Entry model and status types
  sync/                      # Background sync worker
  systray/                   # Ubuntu top bar widget (build tag: systray)
```

**Storage:** `~/.config/timereg/` (database, credentials, token, daemon log)

## Build

```bash
go build -o timereg ./cmd/timereg/              # Without systray
go build -tags systray -o timereg ./cmd/timereg/ # With systray
./install.sh                                     # Full install
```

## How Time Registration Works

### Entry Types

| Type       | Description                        | Has Name | Has Times |
|------------|------------------------------------|----------|-----------|
| assignment | A work task                        | Yes      | Yes       |
| lunch      | Lunch break                        | No       | Yes       |
| break      | Break (fitness, errand, etc.)      | No       | Yes       |
| holiday    | Full day off                       | No       | No        |

### Core Commands

| Command                            | What it does                                                  |
|------------------------------------|---------------------------------------------------------------|
| `timereg start <name>`             | Close any open entry at current time, start new assignment    |
| `timereg lunch <from> <to>`        | Register lunch, auto-splits overlapping assignments           |
| `timereg break`                    | Start a break (closes open entry). Needs next action to close |
| `timereg end`                      | End the day, close open entry, print summary                  |
| `timereg note <text>`              | Add timestamped note to current open entry                    |
| `timereg status`                   | Show today + current week summary                             |
| `timereg week`                     | Full week summary (Mon-Sun)                                   |
| `timereg backfill`                 | Interactive TUI for adding past entries                       |
| `timereg backlog <day> <month> <time> <name>` | Quick-add a past entry                          |
| `timereg edit`                     | Interactive editor for existing entries                       |
| `timereg holiday [d/m]`            | Mark a day as holiday (deletes all entries for that day)      |
| `timereg sync`                     | Manually sync unposted entries to Sheets/Calendar             |
| `timereg purge <from d/m> <to d/m>`| Delete entries from DB, Calendar, and Sheets                  |
| `timereg daemon`                   | Run systray widget + background sync (1 hour interval)        |
| `timereg config`                   | Show/set configuration                                        |
| `timereg auth`                     | Force re-authentication with Google                           |
| `timereg guide`                    | Display Google Cloud setup instructions                       |
| `timereg` (no args)               | Interactive TUI menu                                          |

### Daily Workflow Example

```bash
timereg start "Project Alpha"    # Start working at current time
timereg lunch 12:00 12:30        # Splits "Project Alpha" around lunch automatically
timereg start "Code Review"      # Switch task (closes previous at current time)
timereg end                      # End day, prints work/lunch/break summary
```

### Notes

- Notes are per-entry and start empty when an entry is created
- Add notes only to the currently open (active) entry via `timereg note "text"`
- Each note gets a timestamp in `HH:MM DD/MM: text` format (e.g., `12:02 12/3: Fixed login bug`)
- Multiple notes are joined with newlines in a single field
- Notes are stored in the DB and synced to Google Sheets (column H), not to Calendar
- Notes show in `timereg status` output for the current open entry
- Also available via the TUI options menu and systray

### Key Behaviors

- **Starting an assignment** closes any currently open entry at the new start time.
- **Lunch auto-splits**: If you have an open or closed assignment that overlaps the lunch window, it gets split into before-lunch and after-lunch entries automatically.
- **Break** works like starting an assignment - it closes the open entry. The break itself stays open until the next action.
- **Holiday** deletes all entries for that date and inserts a single holiday entry with no times.
- **Edit** lets you modify name, start time, or end time. It recalculates duration, resolves overlaps with other entries on the same day, resets sync flags, and re-syncs.

### Time Format

- Times can be entered as `830` or `08:30` (both become `08:30`)
- Dates use `d/m` format (e.g., `3/4` = April 3rd of current year)
- All times are in Europe/Copenhagen timezone

### Data Model

Each entry has: ID, Date (YYYY-MM-DD), EntryType, Name, StartTime (HH:MM), EndTime (HH:MM), DurationMinutes, Notes, PostedToSheets, PostedToCalendar.

An entry is "open" when EndTime is empty - it gets closed by the next action.

### Hours Calculation

- Duration = end - start in minutes (handles midnight crossing)
- Hours displayed rounded to 2 decimals
- Only assignment entries count as "work" hours
- Lunch and break minutes are tracked separately

## Google Integration

### Sheets

- One tab per month (named `2026-03`, etc.)
- On sync, the entire month tab is cleared and rebuilt from DB
- Layout: each date gets its own section with entries, daily summary row, and weekly/monthly totals using SUMPRODUCT formulas
- Columns: Date | Day | Type | Name | Start | End | Hours | Notes

### Calendar

- One event per completed entry (must have EndTime)
- Event title = assignment name, or "Lunch"/"Break"
- Timezone: Europe/Copenhagen

### Sync Flags

- Each entry has `posted_to_sheets` and `posted_to_calendar` flags
- Sheets sync triggers for any entry with a start time
- Calendar sync triggers only for completed entries (with end time)
- Flags reset when an entry is edited, causing re-sync

### Config Keys (stored in DB)

- `spreadsheet_id` - Google Sheets ID (accepts full URL)
- `calendar_id` - Google Calendar ID
- `timezone` - IANA timezone (default: Europe/Copenhagen)

## Maintenance Commands

- `timereg purge <from> <to>` - Deletes entries from DB, Calendar events, and rebuilds affected Sheets tabs
- `timereg config create-spreadsheet <name>` / `config create-calendar <name>` - Create new Google resources
- `timereg autostart enable/disable/status` - Manage daemon autostart (.desktop file)
