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

	externalDataRepository := repository.EsiRepositoryWithCache{
		Cache: repository.NewRedisRepository(client),
		Esi:   repository.EsiRepository{},
	}

	orderUseCase := domain.NewOrderUseCase(&externalOrderRepo, orderRepo, &externalDataRepository)

	orderUseCase.IndexOrdersForRegionAndTypeId(10000032, 648)

	defer func() {
		if err := client.Close(); err != nil {
			log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
		}
	}()
}
