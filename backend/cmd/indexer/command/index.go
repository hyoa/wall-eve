package indexer_cmd

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
	Short: "Read stream to index data",
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
			log.Infoln("Listen")

			var idToCheck string
			if checkBackLog {
				idToCheck = "0"
			} else {
				idToCheck = ">"
			}

			args := goredis.XReadGroupArgs{
				Streams:  []string{"indexationAdd", idToCheck},
				Count:    100,
				Block:    2 * time.Second,
				Group:    "indexationAddGroup",
				Consumer: args[0],
			}
			res, _ := client.XReadGroup(context.Background(), &args).Result()

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
					go runIndexation(c, messages[i], &orderUseCase, &itemUseCase, client)
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

func runIndexation(c chan chanIndex, message goredis.XMessage, useCase *domain.OrderUseCase, itemUseCase *domain.ItemUseCase, client *goredis.Client) {
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

	useCase.IndexOrdersForRegionAndTypeId(index.regionId, index.typeId)
	itemUseCase.SetItemAsIndexedForRegionId(index.regionId, index.typeId)

	log.Infoln(message.ID)

	_, errAck := client.XAck(context.Background(), "indexationAdd", "indexationAddGroup", message.ID).Result()

	if errAck != nil {
		log.Errorln(errAck)
	}

	_, errDel := client.XDel(context.Background(), "indexationAdd", message.ID).Result()

	if errDel != nil {
		log.Errorln(errDel)
	}

	c <- chanIndex{}
}
