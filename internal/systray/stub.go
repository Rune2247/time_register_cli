//go:build !systray

package systray

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
)

func Run(d *db.DB) {
	fmt.Println("Systray support is not available in this build.")
	fmt.Println("")
	fmt.Println("To build with systray support, install the required system packages:")
	fmt.Println("  sudo apt install libayatana-appindicator3-dev libgtk-3-dev")
	fmt.Println("")
	fmt.Println("Then rebuild with the systray build tag:")
	fmt.Println("  go build -tags systray ./cmd/timereg/")
}
