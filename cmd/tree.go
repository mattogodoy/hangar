package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mattogodoy/hangar/internal/aerospace"
	"github.com/spf13/cobra"
)

const (
	bold  = "\x1b[1m"
	reset = "\x1b[0m"
)

type winInfo struct {
	id     int
	app    string
	title  string
	parent string
	root   string
	frame  *aerospace.Frame
}

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display the workspace layout tree",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		windows, err := aerospace.ListWindows()
		if err != nil {
			return fmt.Errorf("listing windows: %w", err)
		}

		workspaces, err := aerospace.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("listing workspaces: %w", err)
		}

		monitors, err := aerospace.ListMonitors()
		if err != nil {
			return fmt.Errorf("listing monitors: %w", err)
		}

		showAll, _ := cmd.Flags().GetBool("all")

		monitorWorkspaces := make(map[string][]string)
		for _, ws := range workspaces {
			monitorWorkspaces[ws.MonitorName] = append(monitorWorkspaces[ws.MonitorName], ws.Workspace)
		}

		wsByWs := make(map[string][]winInfo)
		for _, w := range windows {
			wi := winInfo{
				id:     w.WindowID,
				app:    w.AppName,
				title:  w.WindowTitle,
				parent: w.ParentContainerLayout,
				root:   w.RootContainerLayout,
			}
			if f, err := aerospace.GetWindowFrame(w.WindowID); err == nil {
				wi.frame = f
			}
			wsByWs[w.Workspace] = append(wsByWs[w.Workspace], wi)
		}

		sort.Slice(monitors, func(i, j int) bool {
			return monitors[i].MonitorID < monitors[j].MonitorID
		})

		for mi, mon := range monitors {
			if mi > 0 {
				fmt.Println()
			}
			label := fmt.Sprintf(" %s ", mon.MonitorName)
			bar := strings.Repeat("━", len(label))
			fmt.Printf("┏%s┓\n", bar)
			fmt.Printf("┃%s┃\n", label)
			fmt.Printf("┗%s┛\n", bar)

			wsList := monitorWorkspaces[mon.MonitorName]
			sort.Strings(wsList)

			// Filter empty workspaces unless --all
			if !showAll {
				var nonempty []string
				for _, ws := range wsList {
					if len(wsByWs[ws]) > 0 {
						nonempty = append(nonempty, ws)
					}
				}
				wsList = nonempty
			}

			if len(wsList) == 0 {
				fmt.Printf("  (no active workspaces)\n")
				continue
			}

			for wi, ws := range wsList {
				wsPrefix := "├"
				wsChildPrefix := "│"
				if wi == len(wsList)-1 {
					wsPrefix = "└"
					wsChildPrefix = " "
				}

				wins := wsByWs[ws]
				if len(wins) == 0 {
					fmt.Printf("  %s── %sworkspace %s%s (empty)\n", wsPrefix, bold, ws, reset)
					continue
				}

				rootLayout := wins[0].root
				fmt.Printf("  %s── %sworkspace %s%s [%s]\n", wsPrefix, bold, ws, reset, rootLayout)

				printWorkspaceTree(wins, rootLayout, "  "+wsChildPrefix)
			}
		}

		return nil
	},
}

func printWorkspaceTree(wins []winInfo, rootLayout, prefix string) {
	// Build tree nodes: root-level windows + sub-groups.
	// Group ALL windows with the same non-root parent layout together,
	// regardless of their position in the list.
	type treeNode struct {
		wins   []winInfo
		layout string
		pos    float64
	}

	var rootNodes []treeNode
	subGroups := make(map[string]*treeNode)
	subGroupOrder := []string{}

	for _, w := range wins {
		if w.parent == rootLayout {
			pos := 0.0
			if w.frame != nil {
				if strings.HasPrefix(rootLayout, "v_") {
					pos = w.frame.Y
				} else {
					pos = w.frame.X
				}
			}
			rootNodes = append(rootNodes, treeNode{wins: []winInfo{w}, layout: w.parent, pos: pos})
		} else if w.parent == "floating" {
			rootNodes = append(rootNodes, treeNode{wins: []winInfo{w}, layout: "floating", pos: 9999999})
		} else {
			g, ok := subGroups[w.parent]
			if !ok {
				g = &treeNode{layout: w.parent, pos: 9999998}
				subGroups[w.parent] = g
				subGroupOrder = append(subGroupOrder, w.parent)
			}
			g.wins = append(g.wins, w)
			if w.frame != nil {
				pos := w.frame.X
				if strings.HasPrefix(rootLayout, "v_") {
					pos = w.frame.Y
				}
				if pos < g.pos {
					g.pos = pos
				}
			}
		}
	}

	for _, key := range subGroupOrder {
		rootNodes = append(rootNodes, *subGroups[key])
	}

	sort.Slice(rootNodes, func(i, j int) bool {
		return rootNodes[i].pos < rootNodes[j].pos
	})

	for ni, n := range rootNodes {
		isLast := ni == len(rootNodes)-1
		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		if len(n.wins) == 1 {
			w := n.wins[0]
			sizeStr := sizeLabel(w)
			icon := "▪"
			if w.parent == "floating" {
				icon = "◈"
			}
			fmt.Printf("%s%s%s %s — %s%s\n", prefix, connector, icon, w.app, truncate(w.title, 40), sizeStr)
		} else {
			fmt.Printf("%s%s┬ [%s]\n", prefix, connector, n.layout)
			// Sort sub-group windows by the sub-container's axis
			sortSubGroupWins(n.wins, n.layout)
			for ci, w := range n.wins {
				subConn := "├── "
				if ci == len(n.wins)-1 {
					subConn = "└── "
				}
				sizeStr := sizeLabel(w)
				fmt.Printf("%s%s%s▪ %s — %s%s\n", prefix, childPrefix, subConn, w.app, truncate(w.title, 40), sizeStr)
			}
		}
	}
}

func sortSubGroupWins(wins []winInfo, layout string) {
	sort.Slice(wins, func(i, j int) bool {
		fi, fj := wins[i].frame, wins[j].frame
		if fi == nil || fj == nil {
			return false
		}
		if strings.HasPrefix(layout, "v_") {
			return fi.Y < fj.Y
		}
		return fi.X < fj.X
	})
}

func sizeLabel(w winInfo) string {
	if w.frame == nil {
		return ""
	}
	return fmt.Sprintf(" (%.0fx%.0f)", w.frame.W, w.frame.H)
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

func init() {
	treeCmd.Flags().BoolP("all", "a", false, "Show empty workspaces")
	rootCmd.AddCommand(treeCmd)
}
