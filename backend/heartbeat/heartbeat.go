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
	pubsub := c.client.Subscribe(context.Background(), "denormalizedOrdersRegionSearch")

	defer pubsub.Close()

	log.Info("Listen for region to catchup")
	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			log.Error(err.Error())
		}

		v, _ := strconv.Atoi(msg.Payload)
		c.catchupRegionIfNeeded(v)
		c.writeCallHistory(v)
	}
}

func (c *heartbeat) catchupRegionIfNeeded(regionId int) error {
	key := fmt.Sprintf("indexationCatchupLaunch:%d", regionId)
	v, err := c.client.Get(context.Background(), key).Result()

	if (err != nil && err.Error() != "redis: nil") || v == "1" {
		return nil
	}

	now := time.Now().Unix()
	res, _ := c.client.ZRangeByScore(
		context.Background(),
		"indexationDelayed",
		&goredis.ZRangeBy{
			Min: fmt.Sprintf("%d", now),
			Max: fmt.Sprintf("%d", now+300),
		},
	).Result()

	if len(res) == 0 {
		args := goredis.XAddArgs{
			Stream: "indexationCatchup",
			Values: []interface{}{"regionId", regionId},
		}

		log.Infof("Ask to catchup %d", regionId)

		c.client.XAdd(context.Background(), &args)
		c.client.Set(context.Background(), key, true, time.Minute)
	}

	return nil
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
