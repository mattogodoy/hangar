package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hangar",
	Short: "Persist and restore AeroSpace window layouts",
	Long:  "Hangar saves and restores your AeroSpace workspace-to-monitor assignments and window-to-workspace placements across restarts and monitor changes.",
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() error {
	return rootCmd.Execute()
}
