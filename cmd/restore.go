package cmd

import (
	"fmt"
	"strings"

	"github.com/mattogodoy/hangar/internal/aerospace"
	"github.com/mattogodoy/hangar/internal/profile"
	"github.com/spf13/cobra"
)

type matchedWindow struct {
	saved   profile.WindowEntry
	current aerospace.Window
}

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

		// Phase 2: match saved windows to current windows
		currentWindows, err := aerospace.ListWindows()
		if err != nil {
			return fmt.Errorf("listing current windows: %w", err)
		}

		claimed := make(map[int]bool)
		var matches []matchedWindow
		skipped := 0

		for _, saved := range p.Windows {
			best := findBestMatch(currentWindows, saved, claimed)
			if best == nil {
				skipped++
				continue
			}
			claimed[best.WindowID] = true
			matches = append(matches, matchedWindow{saved: saved, current: *best})
		}

		// Phase 3: move windows to their saved workspaces
		moved := 0
		for _, m := range matches {
			if m.current.Workspace != m.saved.Workspace {
				if err := aerospace.MoveWindowToWorkspace(m.current.WindowID, m.saved.Workspace); err != nil {
					fmt.Printf("  warn: %s (%d) -> workspace %s: %v\n", m.current.AppName, m.current.WindowID, m.saved.Workspace, err)
					continue
				}
				moved++
			}
		}
		fmt.Printf("Moved %d windows (%d saved windows had no current match)\n", moved, skipped)

		// Phase 4: restore layout tree per workspace
		wsWindows := make(map[string][]matchedWindow)
		for _, m := range matches {
			wsWindows[m.saved.Workspace] = append(wsWindows[m.saved.Workspace], m)
		}

		for ws, mws := range wsWindows {
			if err := restoreWorkspaceLayout(ws, mws); err != nil {
				fmt.Printf("  warn: layout for workspace %s: %v\n", ws, err)
			}
		}
		fmt.Println("Restored workspace layouts")

		return nil
	},
}

func restoreWorkspaceLayout(workspace string, matches []matchedWindow) error {
	if len(matches) == 0 {
		return nil
	}

	if err := aerospace.FlattenWorkspaceTree(workspace); err != nil {
		return fmt.Errorf("flatten: %w", err)
	}

	rootLayout := matches[0].saved.RootContainerLayout
	if rootLayout == "" {
		return nil
	}

	// Separate floating windows — they're handled last
	var tiling []matchedWindow
	var floating []matchedWindow
	for _, m := range matches {
		if m.saved.ParentContainerLayout == "floating" {
			floating = append(floating, m)
		} else {
			tiling = append(tiling, m)
		}
	}

	// Set root container layout
	if len(tiling) > 0 {
		if err := aerospace.SetLayout(tiling[0].current.WindowID, rootLayout); err != nil {
			return fmt.Errorf("set root layout: %w", err)
		}
	}

	// Find groups of consecutive windows with a different parent layout (sub-containers).
	type subGroup struct {
		windowIDs []int
		layout    string
	}
	var groups []subGroup
	var currentGroup *subGroup

	for _, m := range tiling {
		parentLayout := m.saved.ParentContainerLayout
		if parentLayout == rootLayout {
			if currentGroup != nil {
				groups = append(groups, *currentGroup)
				currentGroup = nil
			}
			continue
		}
		if currentGroup != nil && currentGroup.layout == parentLayout {
			currentGroup.windowIDs = append(currentGroup.windowIDs, m.current.WindowID)
		} else {
			if currentGroup != nil {
				groups = append(groups, *currentGroup)
			}
			currentGroup = &subGroup{
				windowIDs: []int{m.current.WindowID},
				layout:    parentLayout,
			}
		}
	}
	if currentGroup != nil {
		groups = append(groups, *currentGroup)
	}

	// Reconstruct sub-containers using join-with.
	// After flatten, windows are children of the root in tree order.
	// join-with creates a new parent container for a window and its neighbor.
	// We try both axis directions since visual position isn't guaranteed.
	for _, g := range groups {
		if len(g.windowIDs) < 2 {
			if err := aerospace.SetLayout(g.windowIDs[0], g.layout); err != nil {
				fmt.Printf("  warn: layout for window %d: %v\n", g.windowIDs[0], err)
			}
			continue
		}

		directions := joinDirections(rootLayout)

		// Join the first window with its neighbor to create a sub-container
		if err := aerospace.TryJoinWith(g.windowIDs[0], directions...); err != nil {
			fmt.Printf("  warn: %v\n", err)
			continue
		}

		// Join remaining windows into the sub-container
		for _, wid := range g.windowIDs[2:] {
			if err := aerospace.TryJoinWith(wid, directions...); err != nil {
				fmt.Printf("  warn: %v\n", err)
			}
		}

		// Set the sub-container's layout
		if err := aerospace.SetLayout(g.windowIDs[0], g.layout); err != nil {
			fmt.Printf("  warn: sub-container layout: %v\n", err)
		}
	}

	// Handle floating windows
	for _, m := range floating {
		if err := aerospace.SetLayout(m.current.WindowID, "floating"); err != nil {
			fmt.Printf("  warn: float window %d: %v\n", m.current.WindowID, err)
		}
	}

	return nil
}

func joinDirections(rootLayout string) []string {
	if strings.HasPrefix(rootLayout, "v_") {
		return []string{"up", "down"}
	}
	return []string{"left", "right"}
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
