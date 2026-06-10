package cmd

import (
	"fmt"
	"strings"

	"github.com/mattogodoy/hangar/internal/aerospace"
	"github.com/mattogodoy/hangar/internal/profile"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore <profile-name>",
	Short: "Restore a saved window layout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		p, err := profile.Load(name)
		if err != nil {
			return err
		}

		currentMonitors, err := aerospace.ListMonitors()
		if err != nil {
			return fmt.Errorf("listing monitors: %w", err)
		}
		availableMonitors := make(map[string]bool)
		for _, m := range currentMonitors {
			availableMonitors[m.MonitorName] = true
		}

		// Phase 1: restore workspace-to-monitor assignments
		for _, ml := range p.Monitors {
			if !availableMonitors[ml.MonitorName] {
				fmt.Printf("  skip: monitor %q not connected\n", ml.MonitorName)
				continue
			}
			for _, ws := range ml.Workspaces {
				if err := aerospace.MoveWorkspaceToMonitor(ws, ml.MonitorName); err != nil {
					fmt.Printf("  warn: workspace %s -> %s: %v\n", ws, ml.MonitorName, err)
				}
			}
		}
		fmt.Println("Restored workspace-to-monitor assignments")

		// Phase 2: restore window-to-workspace assignments
		currentWindows, err := aerospace.ListWindows()
		if err != nil {
			return fmt.Errorf("listing current windows: %w", err)
		}

		claimed := make(map[int]bool)
		moved := 0
		skipped := 0

		for _, saved := range p.Windows {
			best := findBestMatch(currentWindows, saved, claimed)
			if best == nil {
				skipped++
				continue
			}
			claimed[best.WindowID] = true
			if best.Workspace == saved.Workspace {
				continue
			}
			if err := aerospace.MoveWindowToWorkspace(best.WindowID, saved.Workspace); err != nil {
				fmt.Printf("  warn: %s (%d) -> workspace %s: %v\n", best.AppName, best.WindowID, saved.Workspace, err)
				continue
			}
			moved++
		}

		fmt.Printf("Restored %d windows (%d saved windows had no current match)\n", moved, skipped)
		return nil
	},
}

func findBestMatch(current []aerospace.Window, saved profile.WindowEntry, claimed map[int]bool) *aerospace.Window {
	var bundleMatches, nameMatches []aerospace.Window

	for _, w := range current {
		if claimed[w.WindowID] {
			continue
		}
		if saved.AppBundle != "" && w.AppBundle == saved.AppBundle {
			bundleMatches = append(bundleMatches, w)
		} else if strings.EqualFold(w.AppName, saved.AppName) {
			nameMatches = append(nameMatches, w)
		}
	}

	matches := bundleMatches
	if len(matches) == 0 {
		matches = nameMatches
	}
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		return &matches[0]
	}

	for i, w := range matches {
		if w.WindowTitle == saved.Title {
			return &matches[i]
		}
	}

	for i, w := range matches {
		if strings.Contains(w.WindowTitle, saved.Title) || strings.Contains(saved.Title, w.WindowTitle) {
			return &matches[i]
		}
	}

	return &matches[0]
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
