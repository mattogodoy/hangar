package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattogodoy/hangar/internal/profile"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved profiles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := profile.List()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("No saved profiles")
			return nil
		}

		detailed, _ := cmd.Flags().GetBool("detail")

		for _, name := range names {
			if !detailed {
				fmt.Println(name)
				continue
			}
			p, err := profile.Load(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s: error loading: %v\n", name, err)
				continue
			}
			fmt.Printf("%s  (saved %s, %d windows, %d monitors)\n",
				p.Name, p.SavedAt.Format("2006-01-02 15:04"), len(p.Windows), len(p.Monitors))
			if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
				data, _ := json.MarshalIndent(p, "  ", "  ")
				fmt.Printf("  %s\n", data)
			}
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolP("detail", "d", false, "Show profile details")
	listCmd.Flags().BoolP("verbose", "v", false, "Show full profile contents")
	rootCmd.AddCommand(listCmd)
}
