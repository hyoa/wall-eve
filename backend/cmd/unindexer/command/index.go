package unindexer_cmd

import (
	"context"
	"os"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/hyoa/wall-eve/backend/internal/repository"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type chanIndex struct{}

func init() {
	checkCmd.MarkFlagRequired("configPath")

	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "run",
	Short: "Read stream to delete index data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		externalOrderRepo := repository.EsiRepository{}

		var addr = os.Getenv("REDIS_ADDR")
		client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})
		orderRepo := repository.NewRedisRepository(client)
		notifier := repository.NewRedisRepository(client)

		externalDataRepository := repository.EsiRepositoryWithCache{
			Cache: repository.NewRedisRepository(client),
			Esi:   repository.EsiRepository{},
		}

		orderUseCase := domain.NewOrderUseCase(&externalOrderRepo, orderRepo, &externalDataRepository, notifier)
		itemUseCase := domain.NewItemUseCase(
			repository.NewRedisRepository(client),
			&repository.EsiRepository{},
			repository.NewRedisRepository(client),
		)

		checkBackLog := true
		for {
			log.Infoln("Listen delete")

			var idToCheck string
			if checkBackLog {
				idToCheck = "0"
			} else {
				idToCheck = ">"
			}

			xReadArgs := goredis.XReadGroupArgs{
				Streams:  []string{"indexationRemove", idToCheck},
				Count:    100,
				Block:    2 * time.Second,
				Group:    "indexationRemoveGroup",
				Consumer: args[0],
			}
			res, _ := client.XReadGroup(context.Background(), &xReadArgs).Result()

			// if errXRead != nil {
			// 	log.Errorln(errXRead)
			// }

			type streamItem struct {
				id     string
				values map[string]string
			}

			c := make(chan chanIndex)
			if len(res) > 0 {
				stream := res[0]
				messages := stream.Messages
				if checkBackLog && len(messages) == 0 {
					checkBackLog = false
				}

				log.Infoln("Indexing ", len(messages), " messages")
				for i := 0; i < len(messages); i++ {
					go runDeleteIndexation(c, messages[i], &orderUseCase, &itemUseCase, client)
				}

			}

			// log.Infoln("End listen")
			defer func() {
				if err := client.Close(); err != nil {
					log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
				}
			}()
		}
	},
}

func runDeleteIndexation(c chan chanIndex, message goredis.XMessage, useCase *domain.OrderUseCase, itemUseCase *domain.ItemUseCase, client *goredis.Client) {
	index := struct {
		regionId, typeId int32
	}{}

	values := message.Values
	for k := range values {
		switch val := values[k].(type) {
		case string:
			if k == "regionId" {
				v, _ := strconv.Atoi(val)
				index.regionId = int32(v)
			}

			if k == "typeId" {
				v, _ := strconv.Atoi(val)
				index.typeId = int32(v)
			}
		}
	}

	useCase.DeleteIndexedOrdersForRegionAndType(int(index.regionId), int(index.typeId))
	itemUseCase.RemoveItemFromIndexForRegionId(index.regionId, index.typeId)

	client.XAck(context.Background(), "indexationRemove", "indexationRemoveGroup", message.ID).Result()
	client.XDel(context.Background(), "indexationRemove", message.ID)
	c <- chanIndex{}
}
