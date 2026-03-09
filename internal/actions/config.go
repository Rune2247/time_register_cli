package actions

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rlf/time_register_cli/internal/db"
)

// Known config keys
const (
	ConfigSpreadsheetID  = "spreadsheet_id"
	ConfigCalendarID     = "calendar_id"
	ConfigDefaultLunch   = "default_lunch_time"
	ConfigDefaultEnd     = "default_end_time"
	ConfigTimezone        = "timezone"
)

var configDescriptions = map[string]string{
	ConfigSpreadsheetID: "Google Spreadsheet ID",
	ConfigCalendarID:    "Google Calendar ID",
	ConfigDefaultLunch:  "Default lunch time (HH:MM)",
	ConfigDefaultEnd:    "Default end-of-day time (HH:MM)",
	ConfigTimezone:      "Timezone",
}

// spreadsheetIDRegex extracts the ID from a Google Sheets URL.
// Matches: https://docs.google.com/spreadsheets/d/SPREADSHEET_ID/...
var spreadsheetIDRegex = regexp.MustCompile(`/spreadsheets/d/([a-zA-Z0-9_-]+)`)

func GetConfigValue(d *db.DB, key string) (string, error) {
	return d.GetConfig(key)
}

func SetConfigValue(d *db.DB, key, value string) error {
	value = normalizeConfigValue(key, value)

	if err := d.SetConfig(key, value); err != nil {
		return err
	}
	desc := configDescriptions[key]
	if desc == "" {
		desc = key
	}
	fmt.Printf("%s = %s\n", desc, value)
	return nil
}

// normalizeConfigValue extracts IDs from URLs when applicable.
func normalizeConfigValue(key, value string) string {
	switch key {
	case ConfigSpreadsheetID:
		if matches := spreadsheetIDRegex.FindStringSubmatch(value); len(matches) > 1 {
			return matches[1]
		}
	case ConfigCalendarID:
		// Strip any URL wrapping, just keep the calendar ID
		value = strings.TrimSpace(value)
	}
	return value
}

func PrintAllConfig(d *db.DB) error {
	config, err := d.GetAllConfig()
	if err != nil {
		return err
	}

	if len(config) == 0 {
		fmt.Println("No configuration set. Run 'timereg config setup' to get started.")
		return nil
	}

	fmt.Println("Configuration:")
	// Print known keys first in order
	knownKeys := []string{ConfigSpreadsheetID, ConfigCalendarID, ConfigDefaultLunch, ConfigDefaultEnd, ConfigTimezone}
	printed := make(map[string]bool)
	for _, k := range knownKeys {
		if v, ok := config[k]; ok {
			desc := configDescriptions[k]
			fmt.Printf("  %-30s %s\n", desc+":", v)
			printed[k] = true
		}
	}
	// Print any remaining keys
	for k, v := range config {
		if !printed[k] {
			fmt.Printf("  %-30s %s\n", k+":", v)
		}
	}
	return nil
}
