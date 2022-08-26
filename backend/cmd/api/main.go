package main

import (
	"os"

	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/controller"
)

func main() {
	var addr = os.Getenv("REDIS_ADDR")
	client := goredis.NewClient(&goredis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASSWORD")})

	c := controller.NewOrderController(client)

	r := gin.Default()
	r.GET("/market", c.GetDenormOrdersWithFilter)

	r.Run(":1337")
}
