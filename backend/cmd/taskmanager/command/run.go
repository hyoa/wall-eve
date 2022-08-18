package taskmanager_cmd

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type chanIndex struct{}

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "run",
	Short: "Read stream to delete index data",
	Run: func(cmd *cobra.Command, args []string) {
		var addr = os.Getenv("REDIS_ADDR")
		client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})

		res, _ := client.ZRangeWithScores(context.Background(), "indexationDelayed", 0, 0).Result()

		var regionId, typeId int
		if res[0].Score <= float64(time.Now().Unix()) {
			switch val := res[0].Member.(type) {
			case string:
				content := strings.Split(val, ":")

				regionId, _ = strconv.Atoi(content[0])
				typeId, _ = strconv.Atoi(content[1])
			}

			if regionId != 0 && typeId != 0 {
				log.Infoln("Found 1 item to index")
				xAddArgs := goredis.XAddArgs{
					Stream: "indexationAdd",
					Values: []interface{}{"regionId", regionId, "typeId", typeId},
				}
				client.XAdd(context.Background(), &xAddArgs)
			}

			client.ZRem(context.Background(), "indexationDelayed", res[0].Member)
		}

		// log.Infoln("End listen")
		defer func() {
			if err := client.Close(); err != nil {
				log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
			}
		}()
	},
}
