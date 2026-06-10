package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type WindowEntry struct {
	AppName               string `json:"app_name"`
	AppBundle             string `json:"app_bundle_id"`
	Title                 string `json:"title"`
	Workspace             string `json:"workspace"`
	ParentContainerLayout string `json:"parent_container_layout"`
	RootContainerLayout   string `json:"root_container_layout"`
}

type MonitorLayout struct {
	MonitorName string   `json:"monitor_name"`
	Workspaces  []string `json:"workspaces"`
}

type Profile struct {
	Name      string          `json:"name"`
	SavedAt   time.Time       `json:"saved_at"`
	Monitors  []MonitorLayout `json:"monitors"`
	Windows   []WindowEntry   `json:"windows"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "hangar", "profiles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func filePath(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

func Save(p *Profile) error {
	path, err := filePath(p.Name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Load(name string) (*Profile, error) {
	path, err := filePath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile %q not found", name)
		}
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing profile %q: %w", name, err)
	}
	return &p, nil
}

func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(names)
	return names, nil
}

func Delete(name string) error {
	path, err := filePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile %q not found", name)
		}
		return err
	}
	return nil
}
