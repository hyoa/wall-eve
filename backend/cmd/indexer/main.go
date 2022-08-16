package main

import (
	"context"
	"os"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/hyoa/wall-eve/backend/internal/repository"
	log "github.com/sirupsen/logrus"
)

type chanIndex struct{}

func main() {
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

	for {
		// log.Infoln("Listen")
		args := goredis.XReadArgs{Streams: []string{"indexation", "0"}, Count: 100, Block: 2 * time.Second}
		res, _ := client.XRead(context.Background(), &args).Result()

		type streamItem struct {
			id     string
			values map[string]string
		}

		c := make(chan chanIndex)
		if len(res) > 0 {
			stream := res[0]
			messages := stream.Messages

			for i := 0; i < len(messages); i++ {
				go runIndexation(c, messages[i], &orderUseCase, client)
			}

		}

		// log.Infoln("End listen")
		defer func() {
			if err := client.Close(); err != nil {
				log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
			}
		}()
	}
}

func runIndexation(c chan chanIndex, message goredis.XMessage, useCase *domain.OrderUseCase, client *goredis.Client) {
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

	client.XDel(context.Background(), "indexation", message.ID).Result()
	c <- chanIndex{}
}
