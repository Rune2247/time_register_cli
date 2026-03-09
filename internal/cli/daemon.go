package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rlf/time_register_cli/internal/db"
	trayPkg "github.com/rlf/time_register_cli/internal/systray"
	"github.com/spf13/cobra"
)

func newDaemonCmd(d *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the systray daemon (background sync + top bar widget)",
		Long: `Run TimeReg as a background daemon with:
  - Ubuntu top bar systray widget
  - Background sync worker (every 5 minutes)
  - Desktop notifications for actions

The systray shows your current assignment and elapsed time.
Click it to start assignments, take lunch, or end the day.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			trayPkg.Run(d)
			return nil
		},
	}
}

func newAutostartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autostart",
		Short: "Manage autostart on login",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Enable TimeReg daemon to start on login",
		RunE: func(cmd *cobra.Command, args []string) error {
			return enableAutostart()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable TimeReg daemon from starting on login",
		RunE: func(cmd *cobra.Command, args []string) error {
			return disableAutostart()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check if autostart is enabled",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := autostartPath()
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("Autostart is enabled (%s)\n", path)
			} else {
				fmt.Println("Autostart is disabled")
			}
			return nil
		},
	})

	return cmd
}

func autostartPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", "timereg.desktop")
}

func enableAutostart() error {
	// Find the timereg binary
	binaryPath, err := exec.LookPath("timereg")
	if err != nil {
		// Fallback: use the current executable
		binaryPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("could not find timereg binary: %w", err)
		}
	}

	desktopEntry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=TimeReg
Comment=Time registration daemon
Exec=%s daemon
Icon=clock
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
`, binaryPath)

	autostartDir := filepath.Dir(autostartPath())
	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}

	if err := os.WriteFile(autostartPath(), []byte(desktopEntry), 0644); err != nil {
		return fmt.Errorf("write autostart file: %w", err)
	}

	fmt.Printf("Autostart enabled: %s\n", autostartPath())
	fmt.Println("TimeReg daemon will start on next login.")
	return nil
}

func disableAutostart() error {
	path := autostartPath()
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Autostart was not enabled.")
			return nil
		}
		return fmt.Errorf("remove autostart file: %w", err)
	}

	fmt.Println("Autostart disabled.")
	return nil
}
