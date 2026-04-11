// internal/cli/version.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Example: `  proof version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("proof %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
		},
	})
}
