package cli

import (
	"context"
	"os"

	goredis "github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	installCmd.Flags().StringVarP((&envFile), "envFile", "e", "", "env file location")
	rootCmd.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Create require informations for redis",
	Run: func(cmd *cobra.Command, args []string) {
		if envFile != "" {
			err := loadEnv()

			if err != nil {
				return
			}
		}

		var addr = os.Getenv("REDIS_ADDR")
		client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})

		_, errCreateIdx := client.Do(
			context.Background(),
			"FT.CREATE", "denormalizedOrdersIdx",
			"ON", "JSON",
			"PREFIX", "1", "denormalizedOrders:",
			"SCHEMA",
			"$.regionId", "AS", "regionId", "NUMERIC",
			"$.systemId", "AS", "systemId", "NUMERIC",
			"$.locationId", "AS", "locationId", "NUMERIC",
			"$.typeId", "AS", "typeId", "NUMERIC",
			"$.buyPrice", "AS", "buyPrice", "NUMERIC",
			"$.sellPrice", "AS", "sellPrice", "NUMERIC",
			"$.locationName", "AS", "locationName", "TEXT",
			"$.regionName", "AS", "regionName", "TEXT",
			"$.systemName", "AS", "systemName", "TEXT",
			"$.typeName", "AS", "typeName", "TEXT",
			"$.locationNameConcat", "AS", "locationNameConcat", "TEXT",
			"$.locationTags", "AS", "locationTags", "TAG", "SEPARATOR", ",",
		).Result()

		if errCreateIdx != nil {
			log.Errorln(errCreateIdx.Error())
		} else {
			log.Infoln("Index denormalizedOrdersIdx created")
		}

		_, errCreateXGroup := client.XGroupCreateMkStream(context.Background(), "indexationAdd", "indexationAddGroup", "0").Result()

		if errCreateXGroup != nil {
			log.Errorln(errCreateXGroup.Error())
		} else {
			log.Infoln("Stream group created")
		}
	},
}
