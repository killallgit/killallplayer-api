package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build variables - these will be set during build time using ldflags
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	GoVersion = runtime.Version()
	OS        = runtime.GOOS
	Arch      = runtime.GOARCH
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long: `Display detailed version information about the Podcast Player API.

This includes the version number, git commit hash, build time,
and runtime information.`,
	Run: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolP("short", "s", false, "print just the version number")
}

func runVersion(cmd *cobra.Command, args []string) {
	short, _ := cmd.Flags().GetBool("short")

	if short {
		fmt.Fprintf(cmd.OutOrStdout(), "v%s\n", Version)
		return
	}

	// Print detailed version information
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Podcast Player API")
	fmt.Fprintln(out, repeatString("-", 40))
	fmt.Fprintf(out, "Version:      v%s\n", Version)
	fmt.Fprintf(out, "Git Commit:   %s\n", GitCommit)
	fmt.Fprintf(out, "Build Time:   %s\n", BuildTime)
	fmt.Fprintf(out, "Go Version:   %s\n", GoVersion)
	fmt.Fprintf(out, "OS/Arch:      %s/%s\n", OS, Arch)
	fmt.Fprintln(out, repeatString("-", 40))
}
