package cli

import (
	"context"
	"os"
	"strconv"

	goredis "github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(warmupCmd)
}

var warmupCmd = &cobra.Command{
	Use:   "warmup",
	Short: "Create require informations for redis",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if envFile != "" {
			err := loadEnv()

			if err != nil {
				return
			}
		}

		var addr = os.Getenv("REDIS_ADDR")
		client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})

		regionId, _ := strconv.Atoi(args[0])

		xAddArgs := goredis.XAddArgs{
			Stream: "indexationAdd",
			Values: []interface{}{"regionId", regionId},
		}
		client.XAdd(context.Background(), &xAddArgs)

		log.Infoln("Ask indexation for ", regionId)
	},
}
