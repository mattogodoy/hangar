package cmd

import (
	"fmt"

	"github.com/mattogodoy/hangar/internal/profile"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <profile-name>",
	Short: "Delete a saved profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := profile.Delete(name); err != nil {
			return err
		}
		fmt.Printf("Deleted profile %q\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
