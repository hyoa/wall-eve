package main

import (
	"log"
	"os"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/hyoa/wall-eve/backend/internal/repository"
)

func main() {
	externalOrderRepo := repository.EsiRepository{}
	var addr = os.Getenv("REDIS_ADDR")
	client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})
	orderRepo := repository.NewRedisRepository(client)
	externalDataRepo := repository.EsiRepositoryWithCache{}
	notifier := repository.NewRedisRepository(client)

	orderUseCase := domain.NewOrderUseCase(&externalOrderRepo, orderRepo, &externalDataRepo, notifier)

	orderUseCase.FetchAllOrdersForRegion(10000002)

	defer func() {
		if err := client.Close(); err != nil {
			log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
		}
	}()
}
