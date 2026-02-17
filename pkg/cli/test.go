package cli

import (
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Manage test execution",
	Long:  `Schedule, monitor, and inspect test runs.`,
}

func init() {
	rootCmd.AddCommand(testCmd)
}
