package main

import (
	"os"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/hyoa/wall-eve/backend/internal/repository"
)

func main() {
	var addr = os.Getenv("REDIS_ADDR")
	client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})
	itemRepository := repository.NewRedisRepository(client)
	externalItemRepository := repository.EsiRepository{}
	notifier := repository.NewRedisRepository(client)

	itemUseCase := domain.NewItemUseCase(itemRepository, &externalItemRepository, notifier)
	itemUseCase.FindItemToFetch(10000032)
}
