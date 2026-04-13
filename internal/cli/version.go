// internal/cli/version.go
package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time. Falls back to debug.BuildInfo for go install.
var (
	Version   = ""
	Commit    = ""
	BuildDate = ""
)

func init() {
	// If ldflags weren't set, try to get version from Go module info
	if Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				Version = info.Main.Version
			}
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && Commit == "" {
					if len(s.Value) > 7 {
						Commit = s.Value[:7]
					} else {
						Commit = s.Value
					}
				}
				if s.Key == "vcs.time" && BuildDate == "" {
					BuildDate = s.Value
				}
			}
		}
	}
	if Version == "" {
		Version = "dev"
	}
	if Commit == "" {
		Commit = "none"
	}
	if BuildDate == "" {
		BuildDate = "unknown"
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Example: `  proof version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("proof %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
		},
	})
}
