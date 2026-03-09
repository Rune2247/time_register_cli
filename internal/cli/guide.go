package cli

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
	googleapi "github.com/rlf/time_register_cli/internal/google"
	"github.com/spf13/cobra"
)

const guideText = `
=== Google API Setup Guide ===

TimeReg needs access to Google Sheets and Google Calendar.
Follow these steps to set up your credentials:

STEP 1: Create a Google Cloud Project
  1. Go to https://console.cloud.google.com
  2. Click "Select a project" in the top bar, then "New Project"
  3. Name it something like "TimeReg" and click "Create"
  4. Make sure the new project is selected in the top bar

STEP 2: Enable APIs
  1. Go to https://console.cloud.google.com/apis/library
  2. Search for "Google Sheets API" and click "Enable"
  3. Search for "Google Calendar API" and click "Enable"

STEP 3: Configure OAuth Consent Screen
  1. Go to https://console.cloud.google.com/apis/credentials/consent
  2. Select "External" and click "Create"
  3. Fill in:
     - App name: TimeReg
     - User support email: your email
     - Developer contact: your email
  4. Click "Save and Continue"
  5. On the "Scopes" page, click "Add or Remove Scopes"
  6. Add these scopes:
     - https://www.googleapis.com/auth/spreadsheets
     - https://www.googleapis.com/auth/calendar.events
  7. Click "Save and Continue"
  8. On "Test users", add your Google email
  9. Click "Save and Continue"

STEP 4: Create OAuth Credentials
  1. Go to https://console.cloud.google.com/apis/credentials
  2. Click "Create Credentials" -> "OAuth client ID"
  3. Application type: "Desktop app"
  4. Name: "TimeReg CLI"
  5. Click "Create"
  6. Click "Download JSON" on the popup

STEP 5: Save the Credentials File
  Move the downloaded JSON file to:
    ~/.config/timereg/credentials.json

  Example:
    mv ~/Downloads/client_secret_*.json ~/.config/timereg/credentials.json

STEP 6: Authenticate
  Run:
    timereg auth

  This opens a URL in your terminal. Copy it to your browser,
  sign in with your Google account, and paste the authorization
  code back into the terminal.

STEP 7: Configure Spreadsheet and Calendar
  Option A — Use existing spreadsheet/calendar:
    timereg config set spreadsheet_id "YOUR_SPREADSHEET_URL_OR_ID"
    timereg config set calendar_id "YOUR_CALENDAR_ID"

  Option B — Interactive setup:
    timereg config setup

  To find your Calendar ID:
    Google Calendar -> Settings -> your calendar -> "Integrate calendar"
    It looks like: abc123@group.calendar.google.com
    Or use your email for the primary calendar.

STEP 8: Test
  Run:
    timereg sync

  If everything is set up correctly, it will sync any pending entries.

=== You're all set! ===

Run "timereg config" to see your current configuration.
Run "timereg" to start the interactive menu.

=== Optional: Systray (Ubuntu Top Bar Widget) ===

The systray shows your current assignment and elapsed time in the
Ubuntu top bar. It also runs background sync every 5 minutes.

STEP 1: Install system dependencies
  sudo apt install libayatana-appindicator3-dev libgtk-3-dev

STEP 2: Rebuild with systray support
  go build -tags systray -o timereg ./cmd/timereg/

  Or if you installed via go install:
  go install -tags systray ./cmd/timereg/

STEP 3: Test the daemon
  timereg daemon

  You should see a "TR" icon in the top bar.
  Click it to start assignments, take lunch, or end the day.
  It uses zenity for input dialogs and notify-send for notifications.

STEP 4: Auto-start on login (optional)
  timereg autostart enable

  This creates a .desktop file in ~/.config/autostart/ so the
  daemon starts automatically when you log in.

  To disable:
  timereg autostart disable

  To check status:
  timereg autostart status

=== Security Note ===

Your Google credentials are stored outside the git repository:
  ~/.config/timereg/credentials.json  (OAuth client config)
  ~/.config/timereg/token.json        (your auth token, 0600 permissions)
  ~/.config/timereg/timereg.db        (local SQLite database)

These files are NEVER committed to git. The .gitignore also blocks
any credentials.json or token.json files if accidentally placed in
the project directory.
`

func newGuideCmd(d *db.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Step-by-step setup guide for Google API integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(guideText)
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check if Google integration is configured",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printSetupStatus(d)
		},
	})

	return cmd
}

func printSetupStatus(d *db.DB) error {
	fmt.Println("TimeReg Setup Status")
	fmt.Println("====================")

	// Check credentials file
	credStatus := "not found"
	credPath, err := googleapi.CredentialsFilePath()
	if err == nil {
		if googleapi.CredentialsFileExists() {
			credStatus = "found"
		}
	}
	printStatus("Credentials file", credStatus, credPath)

	// Check auth token
	authStatus := "not authenticated"
	if googleapi.IsAuthenticated() {
		authStatus = "authenticated"
	}
	printStatus("Google auth", authStatus, "")

	// Check spreadsheet
	sheetID, _ := d.GetConfig("spreadsheet_id")
	sheetStatus := "not set"
	if sheetID != "" {
		sheetStatus = sheetID
	}
	printStatus("Spreadsheet ID", sheetStatus, "")

	// Check calendar
	calID, _ := d.GetConfig("calendar_id")
	calStatus := "not set"
	if calID != "" {
		calStatus = calID
	}
	printStatus("Calendar ID", calStatus, "")

	// Check defaults
	lunch, _ := d.GetConfig("default_lunch_time")
	if lunch == "" {
		lunch = "12:00 (default)"
	}
	printStatus("Lunch time", lunch, "")

	endTime, _ := d.GetConfig("default_end_time")
	if endTime == "" {
		endTime = "16:00 (default)"
	}
	printStatus("End-of-day time", endTime, "")

	fmt.Println()
	if credStatus == "not found" {
		fmt.Println("Next step: Run 'timereg guide' for full setup instructions.")
	} else if authStatus == "not authenticated" {
		fmt.Println("Next step: Run 'timereg auth' to authenticate with Google.")
	} else if sheetID == "" || calID == "" {
		fmt.Println("Next step: Run 'timereg config setup' to set spreadsheet and calendar IDs.")
	} else {
		fmt.Println("All set! Run 'timereg' to start.")
	}

	return nil
}

func printStatus(label, status, extra string) {
	icon := "x"
	if status != "not found" && status != "not set" && status != "not authenticated" {
		icon = "v"
	}
	fmt.Printf("  [%s] %-20s %s", icon, label+":", status)
	if extra != "" {
		fmt.Printf(" (%s)", extra)
	}
	fmt.Println()
}
