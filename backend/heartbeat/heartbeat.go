package heartbeat

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type heartbeat struct {
	client *goredis.Client
}

func Create(client *goredis.Client) heartbeat {
	return heartbeat{
		client: client,
	}
}

func (c *heartbeat) Run() {
	pubsub := c.client.Subscribe(context.Background(), "apiEvent")

	defer pubsub.Close()

	log.Info("Listen for region access")
	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			log.Error(err.Error())
		}

		v, _ := strconv.Atoi(msg.Payload)
		log.Infof("Write access for %d", v)
		c.writeCallHistory(v)
	}
}

func (c *heartbeat) writeCallHistory(regionId int) error {
	c.client.Do(
		context.Background(),
		"TS.ADD", fmt.Sprintf("regionFetchHistory:%d", regionId),
		time.Now().UnixMilli(),
		1,
	)

	return nil
}
