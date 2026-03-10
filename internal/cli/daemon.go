package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rlf/time_register_cli/internal/db"
	trayPkg "github.com/rlf/time_register_cli/internal/systray"
	"github.com/spf13/cobra"
)

func newDaemonCmd(d *db.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the systray daemon (background sync + top bar widget)",
		Long: `Run TimeReg as a background daemon with:
  - Ubuntu top bar systray widget
  - Background sync worker (every 5 minutes)
  - Desktop notifications for actions

The systray shows your current assignment and elapsed time.
Click it to start assignments, take lunch, or end the day.

Use 'timereg daemon restart' to kill any existing daemon and start a new one detached.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			trayPkg.Run(d)
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "Kill any running daemon and start a new one detached",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RestartDaemon()
		},
	})

	return cmd
}

// RestartDaemon kills any running timereg daemon and starts a new one detached.
func RestartDaemon() error {
	// Kill existing daemon
	killErr := exec.Command("pkill", "-f", "timereg daemon").Run()
	if killErr == nil {
		fmt.Println("Stopped existing daemon")
		time.Sleep(300 * time.Millisecond)
	}

	// Find timereg binary
	binaryPath, err := exec.LookPath("timereg")
	if err != nil {
		binaryPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("could not find timereg binary — is it installed? %w", err)
		}
	}
	fmt.Printf("Using binary: %s\n", binaryPath)

	// Log file
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".config", "timereg")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log directory %s: %w", logDir, err)
	}
	logFile := filepath.Join(logDir, "daemon.log")

	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", logFile, err)
	}

	proc := exec.Command(binaryPath, "daemon")
	proc.Stdout = lf
	proc.Stderr = lf
	proc.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := proc.Start(); err != nil {
		lf.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}
	pid := proc.Process.Pid
	lf.Close()

	// Verify the process is still running after a brief moment
	time.Sleep(500 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err != nil {
		// Process died — read the log for clues
		logContent, readErr := os.ReadFile(logFile)
		if readErr == nil && len(logContent) > 0 {
			return fmt.Errorf("daemon exited immediately. Log:\n%s", string(logContent))
		}
		return fmt.Errorf("daemon exited immediately (pid %d). Check %s for details", pid, logFile)
	}

	fmt.Printf("Daemon running (pid: %d, log: %s)\n", pid, logFile)
	return nil
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
