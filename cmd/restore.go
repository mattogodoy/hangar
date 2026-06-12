package cmd

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mattogodoy/hangar/internal/aerospace"
	"github.com/mattogodoy/hangar/internal/profile"
	"github.com/spf13/cobra"
)

type matchedWindow struct {
	saved   profile.WindowEntry
	current aerospace.Window
}

const tempWorkspace = "hangar-tmp"

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

		// Phase 4: restore layout tree, order, and sizes per workspace
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

// rootNode represents a root-level element in the workspace tree:
// either a single window or a sub-container group.
type rootNode struct {
	windowIDs []int
	layout    string // only set for sub-groups (differs from root layout)
	pos       float64
}

func restoreWorkspaceLayout(workspace string, matches []matchedWindow) error {
	if len(matches) == 0 {
		return nil
	}

	rootLayout := matches[0].saved.RootContainerLayout
	if rootLayout == "" {
		return nil
	}

	// Separate floating windows
	var tiling []matchedWindow
	var floating []matchedWindow
	for _, m := range matches {
		if m.saved.ParentContainerLayout == "floating" {
			floating = append(floating, m)
		} else {
			tiling = append(tiling, m)
		}
	}

	// Build root-level nodes: individual windows at root + sub-groups.
	// Each node has a position from the saved frame data for ordering.
	nodes := buildRootNodes(tiling, rootLayout)

	// Sort by saved position to establish desired visual order
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].pos < nodes[j].pos
	})

	// Flatten the workspace to start clean
	if err := aerospace.FlattenWorkspaceTree(workspace); err != nil {
		return fmt.Errorf("flatten: %w", err)
	}

	// Establish correct visual order by reinserting windows.
	// Windows moved into a workspace are placed at the RIGHT edge,
	// so we keep the leftmost window and bring the rest back in order.
	allIDs := allWindowIDs(nodes)
	if len(allIDs) > 1 {
		for _, wid := range allIDs[1:] {
			aerospace.MoveWindowToWorkspace(wid, tempWorkspace)
		}
		for _, wid := range allIDs[1:] {
			aerospace.MoveWindowToWorkspace(wid, workspace)
		}
	}

	// Set root container layout
	if len(allIDs) > 0 {
		aerospace.SetLayout(allIDs[0], rootLayout)
	}

	// Reconstruct sub-containers.
	// The ordering step above already placed sub-group windows adjacent to each other
	// (since they come from the same node in allWindowIDs), so we just join them.
	for _, n := range nodes {
		if n.layout == "" || len(n.windowIDs) < 2 {
			if n.layout != "" && len(n.windowIDs) == 1 {
				aerospace.SetLayout(n.windowIDs[0], n.layout)
			}
			continue
		}

		// Join adjacent sub-group windows into a sub-container
		dir := "right"
		if strings.HasPrefix(rootLayout, "v_") {
			dir = "down"
		}
		if err := aerospace.JoinWith(n.windowIDs[0], dir); err != nil {
			opposite := "left"
			if dir == "down" {
				opposite = "up"
			}
			if err2 := aerospace.JoinWith(n.windowIDs[0], opposite); err2 != nil {
				fmt.Printf("  warn: join-with failed for window %d\n", n.windowIDs[0])
				continue
			}
		}

		for i := 2; i < len(n.windowIDs); i++ {
			aerospace.TryJoinWith(n.windowIDs[i], "left", "right", "up", "down")
		}

		aerospace.SetLayout(n.windowIDs[0], n.layout)
	}

	// Restore window sizes (run twice — resizing one window shifts neighbors)
	restoreWindowSizes(tiling, rootLayout)
	restoreWindowSizes(tiling, rootLayout)

	// Handle floating windows
	for _, m := range floating {
		if err := aerospace.SetLayout(m.current.WindowID, "floating"); err != nil {
			fmt.Printf("  warn: float window %d: %v\n", m.current.WindowID, err)
		}
	}

	return nil
}

func buildRootNodes(tiling []matchedWindow, rootLayout string) []rootNode {
	var nodes []rootNode

	// Track which windows belong to sub-groups
	subGroupWindows := make(map[int]bool)
	subGroups := make(map[string]*rootNode)

	for _, m := range tiling {
		pl := m.saved.ParentContainerLayout
		if pl == rootLayout {
			continue
		}
		subGroupWindows[m.current.WindowID] = true
		g, ok := subGroups[pl]
		if !ok {
			g = &rootNode{layout: pl, pos: math.MaxFloat64}
			subGroups[pl] = g
		}
		g.windowIDs = append(g.windowIDs, m.current.WindowID)
		if m.saved.Frame != nil {
			p := posFromFrame(m.saved.Frame, rootLayout)
			if p < g.pos {
				g.pos = p
			}
		}
	}

	// Root-level individual windows
	for _, m := range tiling {
		if subGroupWindows[m.current.WindowID] {
			continue
		}
		pos := 0.0
		if m.saved.Frame != nil {
			pos = posFromFrame(m.saved.Frame, rootLayout)
		}
		nodes = append(nodes, rootNode{
			windowIDs: []int{m.current.WindowID},
			pos:       pos,
		})
	}

	// Add sub-groups as single nodes
	for _, g := range subGroups {
		nodes = append(nodes, *g)
	}

	return nodes
}

func posFromFrame(f *profile.WindowFrame, layout string) float64 {
	if strings.HasPrefix(layout, "v_") {
		return f.Y
	}
	return f.X
}

func allWindowIDs(nodes []rootNode) []int {
	var ids []int
	for _, n := range nodes {
		ids = append(ids, n.windowIDs...)
	}
	return ids
}

func restoreWindowSizes(tiling []matchedWindow, rootLayout string) {
	// First pass: set sizes along the root container axis.
	// In h_ root: set width for each root-level node (windows and sub-groups).
	// In v_ root: set height for each root-level node.
	resized := make(map[int]bool)
	for _, m := range tiling {
		if m.saved.Frame == nil || resized[m.current.WindowID] {
			continue
		}
		if strings.HasPrefix(rootLayout, "h_") {
			w := int(m.saved.Frame.W) + 5
			aerospace.ResizeWindow(m.current.WindowID, "width", w)
		} else if strings.HasPrefix(rootLayout, "v_") {
			h := int(m.saved.Frame.H) + 5
			aerospace.ResizeWindow(m.current.WindowID, "height", h)
		}
		resized[m.current.WindowID] = true
	}

	// Second pass: set sizes along the sub-container axis (the other dimension).
	// Windows in sub-containers need their size set within the sub-container too.
	for _, m := range tiling {
		if m.saved.Frame == nil {
			continue
		}
		pl := m.saved.ParentContainerLayout
		if pl == rootLayout {
			continue
		}
		if strings.HasPrefix(pl, "h_") {
			w := int(m.saved.Frame.W) + 5
			aerospace.ResizeWindow(m.current.WindowID, "width", w)
		} else if strings.HasPrefix(pl, "v_") {
			h := int(m.saved.Frame.H) + 5
			aerospace.ResizeWindow(m.current.WindowID, "height", h)
		}
	}
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
