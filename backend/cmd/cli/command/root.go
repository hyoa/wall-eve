package cli

import "github.com/spf13/cobra"

var (
	rootCmd = &cobra.Command{}
)

func Execute() error {
	return rootCmd.Execute()
}
