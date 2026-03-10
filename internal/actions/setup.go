package actions

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rlf/time_register_cli/internal/db"
)

type setupStep struct {
	key         string
	prompt      string
	description string
	defaultVal  string
}

var setupSteps = []setupStep{
	{
		key:         ConfigSpreadsheetID,
		prompt:      "Google Spreadsheet ID or URL",
		description: "Paste the full Google Sheets URL or just the spreadsheet ID.",
		defaultVal:  "",
	},
	{
		key:         ConfigCalendarID,
		prompt:      "Google Calendar ID",
		description: "Find this in Google Calendar → Settings → calendar → Integrate calendar.\nExample: abc123@group.calendar.google.com (or your email for primary calendar).",
		defaultVal:  "",
	},
	{
		key:         ConfigTimezone,
		prompt:      "Timezone",
		description: "IANA timezone for all time calculations.",
		defaultVal:  "Europe/Copenhagen",
	},
}

func RunSetup(d *db.DB) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("TimeReg Setup")
	fmt.Println("=============")
	fmt.Println("Press Enter to keep the current/default value.")

	for _, step := range setupSteps {
		current, _ := d.GetConfig(step.key)

		fmt.Println(step.description)

		displayDefault := step.defaultVal
		if current != "" {
			displayDefault = current
		}

		if displayDefault != "" {
			fmt.Printf("%s [%s]: ", step.prompt, displayDefault)
		} else {
			fmt.Printf("%s: ", step.prompt)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read input: %w", err)
		}
		input = strings.TrimSpace(input)

		if input == "" {
			if current != "" {
				input = current
			} else {
				input = step.defaultVal
			}
		}

		if input != "" {
			if err := SetConfigValue(d, step.key, input); err != nil {
				return err
			}
		}
		fmt.Println()
	}

	fmt.Println("Setup complete!")
	return nil
}
