package unindexer_cmd

import "github.com/spf13/cobra"

var (
	rootCmd = &cobra.Command{
		Use:   "unindexer",
		Short: "Run an indexer",
	}
)

func Execute() error {
	return rootCmd.Execute()
}
