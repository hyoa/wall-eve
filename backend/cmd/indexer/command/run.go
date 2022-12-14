package indexer_cmd

import (
	"os"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/indexer"
	"github.com/spf13/cobra"
)

type chanIndex struct{}

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "run",
	Short: "Read stream to index data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		var addr = os.Getenv("REDIS_ADDR")
		client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})

		indexer := indexer.Create(client)
		indexer.Run(args[0])
	},
}
