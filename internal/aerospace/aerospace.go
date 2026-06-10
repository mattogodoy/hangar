package aerospace

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type Window struct {
	AppName    string `json:"app-name"`
	AppBundle  string `json:"app-bundle-id"`
	WindowID   int    `json:"window-id"`
	WindowTitle string `json:"window-title"`
	Workspace  string `json:"workspace"`
	MonitorName string `json:"monitor-name"`
}

type Monitor struct {
	MonitorID   int    `json:"monitor-id"`
	MonitorName string `json:"monitor-name"`
}

type WorkspaceMonitor struct {
	Workspace   string `json:"workspace"`
	MonitorName string `json:"monitor-name"`
}

func run(args ...string) ([]byte, error) {
	cmd := exec.Command("aerospace", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("aerospace %v failed: %s", args, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("aerospace %v failed: %w", args, err)
	}
	return out, nil
}

func ListWindows() ([]Window, error) {
	out, err := run(
		"list-windows", "--all", "--json",
		"--format", "%{window-id} %{app-name} %{app-bundle-id} %{window-title} %{workspace} %{monitor-name}",
	)
	if err != nil {
		return nil, err
	}
	var windows []Window
	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("parsing window list: %w", err)
	}
	return windows, nil
}

func ListMonitors() ([]Monitor, error) {
	out, err := run("list-monitors", "--json")
	if err != nil {
		return nil, err
	}
	var monitors []Monitor
	if err := json.Unmarshal(out, &monitors); err != nil {
		return nil, fmt.Errorf("parsing monitor list: %w", err)
	}
	return monitors, nil
}

func ListWorkspaces() ([]WorkspaceMonitor, error) {
	out, err := run(
		"list-workspaces", "--all", "--json",
		"--format", "%{workspace} %{monitor-name}",
	)
	if err != nil {
		return nil, err
	}
	var workspaces []WorkspaceMonitor
	if err := json.Unmarshal(out, &workspaces); err != nil {
		return nil, fmt.Errorf("parsing workspace list: %w", err)
	}
	return workspaces, nil
}

func MoveWorkspaceToMonitor(workspace, monitorPattern string) error {
	_, err := run("move-workspace-to-monitor", "--workspace", workspace, monitorPattern)
	return err
}

func MoveWindowToWorkspace(windowID int, workspace string) error {
	_, err := run("move-node-to-workspace", "--window-id", fmt.Sprintf("%d", windowID), workspace)
	return err
}
