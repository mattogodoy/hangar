package cmd

import (
	"fmt"
	"time"

	"github.com/mattogodoy/hangar/internal/aerospace"
	"github.com/mattogodoy/hangar/internal/profile"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save <profile-name>",
	Short: "Save current window layout to a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		windows, err := aerospace.ListWindows()
		if err != nil {
			return fmt.Errorf("listing windows: %w", err)
		}

		workspaces, err := aerospace.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("listing workspaces: %w", err)
		}

		monitorMap := make(map[string][]string)
		for _, ws := range workspaces {
			monitorMap[ws.MonitorName] = append(monitorMap[ws.MonitorName], ws.Workspace)
		}

		var monitors []profile.MonitorLayout
		for monName, wsList := range monitorMap {
			monitors = append(monitors, profile.MonitorLayout{
				MonitorName: monName,
				Workspaces:  wsList,
			})
		}

		var entries []profile.WindowEntry
		for _, w := range windows {
			entries = append(entries, profile.WindowEntry{
				AppName:   w.AppName,
				AppBundle: w.AppBundle,
				Title:     w.WindowTitle,
				Workspace: w.Workspace,
			})
		}

		p := &profile.Profile{
			Name:     name,
			SavedAt:  time.Now(),
			Monitors: monitors,
			Windows:  entries,
		}

		if err := profile.Save(p); err != nil {
			return fmt.Errorf("saving profile: %w", err)
		}

		fmt.Printf("Saved profile %q: %d windows across %d monitors\n", name, len(entries), len(monitors))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
