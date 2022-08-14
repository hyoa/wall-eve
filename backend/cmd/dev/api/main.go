package main

import (
	"os"

	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/controller"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/hyoa/wall-eve/backend/internal/repository"
)

func main() {
	externalOrderRepo := repository.EsiRepository{}
	var addr = os.Getenv("REDIS_ADDR")
	client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})
	orderRepo := repository.NewRedisRepository(client)
	externalDataRepo := repository.EsiRepositoryWithCache{}

	orderUseCase := domain.NewOrderUseCase(&externalOrderRepo, orderRepo, &externalDataRepo)

	c := controller.NewOrderController(orderUseCase)

	r := gin.Default()
	r.GET("/orders", c.GetOrdersWithFilter)

	r.Run(":1337")
}
