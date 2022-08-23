package scheduler_cmd

import (
	"os"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/scheduler"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type chanIndex struct{}

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "run",
	Short: "Read stream to schedule indexation",
	Run: func(cmd *cobra.Command, args []string) {

		var addr = os.Getenv("REDIS_ADDR")
		client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})
		scheduler := scheduler.Create(client)

		deferedFunc := func() {
			if err := client.Close(); err != nil {
				log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
			}
		}

		scheduler.RunScheduleIndexation(deferedFunc)

		// log.Infoln("End listen")

	},
}
