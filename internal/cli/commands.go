package cli

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/actions"
	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/rlf/time_register_cli/internal/sync"
	"github.com/spf13/cobra"
)

func NewRootCmd(d *db.DB) *cobra.Command {
	root := &cobra.Command{
		Use:   "timereg",
		Short: "TimeReg — CLI time registration tool",
		Long: `TimeReg — CLI time registration tool

Track daily work assignments from your terminal.
Syncs to Google Sheets and Google Calendar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunTUI(d)
		},
	}

	root.AddCommand(
		newStartCmd(d),
		newLunchCmd(d),
		newBreakCmd(d),
		newEndCmd(d),
		newNoteCmd(d),
		newStatusCmd(d),
		newWeekCmd(d),
		newBackfillCmd(d),
		newBacklogCmd(d),
		newEditCmd(d),
		newHolidayCmd(d),
		newPurgeCmd(d),
		newConfigCmd(d),
		newSyncCmd(d),
		newAuthCmd(),
		newGuideCmd(d),
		newDaemonCmd(d),
		newAutostartCmd(),
	)

	return root
}

func newStartCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a new assignment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.StartAssignment(d, actions.TodayInCopenhagen(), actions.NowInCopenhagen(), args[0])
		},
	}
}

func newLunchCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "lunch <from> <to>",
		Short: "Register lunch with from/to times",
		Long: `Register lunch with explicit start and end times.
Splits any overlapping work entries automatically.

Examples:
  timereg lunch 12:00 12:30
  timereg lunch 1200 1230`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := actions.ParseTimeInput(args[0])
			if err != nil {
				return err
			}
			to, err := actions.ParseTimeInput(args[1])
			if err != nil {
				return err
			}
			return actions.InsertLunch(d, actions.TodayInCopenhagen(), from, to)
		},
	}
}

func newBreakCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "break",
		Short: "Start a break (non-work time like fitness, dentist, nap)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.StartBreak(d, actions.TodayInCopenhagen(), actions.NowInCopenhagen())
		},
	}
}

func newEndCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "end",
		Short: "End the day",
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.EndDay(d, actions.TodayInCopenhagen(), actions.NowInCopenhagen())
		},
	}
}

func newNoteCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "note <text>",
		Short: "Add a note to the current active entry",
		Long: `Add a timestamped note to the currently open entry.
If no entry is active, the note is not saved.

Examples:
  timereg note "Fixed login bug"
  timereg note "Reviewed PR #42"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.AddNote(d, args[0])
		},
	}
}

func newStatusCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show today + week summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.PrintStatus(d, actions.TodayInCopenhagen())
		},
	}
}

func newWeekCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "week",
		Short: "Show full week summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			today := actions.TodayInCopenhagen()
			weekStatus, err := actions.GetWeekStatus(d, today)
			if err != nil {
				return err
			}
			fmt.Printf("Week %s to %s\n", weekStatus.StartDate, weekStatus.EndDate)
			fmt.Printf("Work: %.1f hours\n", float64(weekStatus.WorkMinutes)/60.0)
			fmt.Printf("Lunch: %.1f hours\n", float64(weekStatus.LunchMinutes)/60.0)
			if weekStatus.BreakMinutes > 0 {
				fmt.Printf("Break: %.1f hours\n", float64(weekStatus.BreakMinutes)/60.0)
			}
			return nil
		},
	}
}

func newBackfillCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "backfill",
		Short: "Interactive backfill for missed entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunTUI(d)
		},
	}
}

func newBacklogCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "backlog <day> <month> <time> <name|end|break> | backlog <day> <month> lunch <from> <to>",
		Short: "Quick add past entry",
		Long: `Quick add a past entry without prompts.

Examples:
  timereg backlog 3 4 830 "Client meeting"   → assignment on April 3rd at 08:30
  timereg backlog 3 4 1600 end               → end day on April 3rd at 16:00
  timereg backlog 3 4 1400 break             → break on April 3rd at 14:00
  timereg backlog 3 4 lunch 1200 1230        → lunch on April 3rd 12:00-12:30`,
		Args: cobra.RangeArgs(4, 5),
		RunE: func(cmd *cobra.Command, args []string) error {
			date, err := actions.ParseBacklogDate(args[0], args[1])
			if err != nil {
				return err
			}

			// Handle lunch: backlog <day> <month> lunch <from> <to>
			if args[2] == "lunch" {
				if len(args) != 5 {
					return fmt.Errorf("lunch requires from and to times: backlog <day> <month> lunch <from> <to>")
				}
				from, err := actions.ParseTimeInput(args[3])
				if err != nil {
					return err
				}
				to, err := actions.ParseTimeInput(args[4])
				if err != nil {
					return err
				}
				return actions.InsertLunch(d, date, from, to)
			}

			timeStr, err := actions.ParseTimeInput(args[2])
			if err != nil {
				return err
			}

			switch args[3] {
			case "end":
				return actions.EndDay(d, date, timeStr)
			case "break":
				return actions.StartBreak(d, date, timeStr)
			default:
				return actions.StartAssignment(d, date, timeStr, args[3])
			}
		},
	}
}

func newEditCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit existing entries (interactive)",
		Long: `Open the interactive editor to modify or delete past entries.

Select a day, then an entry, then choose what to edit.
Overlapping entries are automatically adjusted.
Edited days are re-synced to Google Calendar and Sheets.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunEditTUI(d)
		},
	}
}

func newHolidayCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "holiday [d/m]",
		Short: "Mark a day as holiday (default: today)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			date := actions.TodayInCopenhagen()
			if len(args) == 1 {
				parsed, err := actions.ParseSlashDate(args[0])
				if err != nil {
					return err
				}
				date = parsed
			}
			return actions.MarkHoliday(d, date)
		},
	}
}

func newPurgeCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "purge <from d/m> <to d/m>",
		Short: "Delete all entries in a date range from SQLite, Calendar, and Sheets",
		Long: `Purge all entries between two dates (inclusive).

Examples:
  timereg purge 1/3 11/3     Purge March 1st to 11th
  timereg purge 15/1 28/2    Purge Jan 15th to Feb 28th`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromDate, err := actions.ParseSlashDate(args[0])
			if err != nil {
				return fmt.Errorf("from date: %w", err)
			}
			toDate, err := actions.ParseSlashDate(args[1])
			if err != nil {
				return fmt.Errorf("to date: %w", err)
			}
			return actions.PurgeTimeRange(d, fromDate, toDate)
		},
	}
}

func newConfigCmd(d *db.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.PrintAllConfig(d)
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.RunSetup(d)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Long: `Set a config value. For spreadsheet_id you can paste the full URL.

Keys: spreadsheet_id, calendar_id, timezone`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.SetConfigValue(d, args[0], args[1])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := actions.GetConfigValue(d, args[0])
			if err != nil {
				return err
			}
			if val == "" {
				fmt.Printf("%s is not set\n", args[0])
			} else {
				fmt.Printf("%s = %s\n", args[0], val)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create-spreadsheet <name>",
		Short: "Create a new Google Spreadsheet and save its ID",
		Long: `Create a new Google Spreadsheet with the given name.
The spreadsheet ID is automatically saved to config.

Example:
  timereg config create-spreadsheet "TimeReg 2026"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fmt.Printf("Creating spreadsheet %q...\n", args[0])
			id, err := googleapi.CreateSpreadsheet(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Spreadsheet created: https://docs.google.com/spreadsheets/d/%s\n", id)
			return actions.SetConfigValue(d, "spreadsheet_id", id)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create-calendar <name>",
		Short: "Create a new Google Calendar and save its ID",
		Long: `Create a new Google Calendar with the given name.
The calendar ID is automatically saved to config.

Example:
  timereg config create-calendar "Work Log"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fmt.Printf("Creating calendar %q...\n", args[0])
			id, err := googleapi.CreateCalendar(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Calendar created: %s\n", id)
			return actions.SetConfigValue(d, "calendar_id", id)
		},
	})

	return cmd
}

func newSyncCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync unposted entries to Google Sheets and Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			sheetID, calID := actions.GetGoogleConfig(d)
			if sheetID == "" && calID == "" {
				return fmt.Errorf("no spreadsheet or calendar configured. Run 'timereg guide status' to check setup")
			}

			entries, err := d.GetUnsyncedEntries()
			if err != nil {
				return fmt.Errorf("check unsynced entries: %w", err)
			}
			if len(entries) == 0 {
				fmt.Println("Everything is up to date — nothing to sync.")
				return nil
			}

			fmt.Printf("Syncing %d entries...\n", len(entries))
			w := sync.NewWorker(d, 0)
			if err := w.SyncNow(); err != nil {
				return err
			}

			remaining, _ := d.GetUnsyncedEntries()
			synced := len(entries) - len(remaining)
			if len(remaining) > 0 {
				fmt.Printf("Synced %d entries, %d failed (will retry on next sync).\n", synced, len(remaining))
			} else {
				fmt.Printf("Synced %d entries successfully.\n", synced)
			}
			return nil
		},
	}
}

func newAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Google (OAuth2)",
		Long: `Run the Google OAuth2 authentication flow.

Prerequisites:
  1. Go to https://console.cloud.google.com
  2. Create a project and enable Google Sheets API + Google Calendar API
  3. Create OAuth2 credentials (Application type: Desktop app)
  4. Download the credentials JSON file
  5. Save it to ~/.config/timereg/credentials.json
  6. Run this command`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return googleapi.RunAuthSetup()
		},
	}
}
