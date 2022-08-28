package refresh

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type refresh struct {
	client *goredis.Client
}

func Create(client *goredis.Client) refresh {
	return refresh{
		client: client,
	}
}

func (r *refresh) Run() {
	pubsub := r.client.Subscribe(context.Background(), "apiEvent")

	defer pubsub.Close()

	log.Info("Listen for region to catchup")
	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			log.Error(err.Error())
		}

		v, _ := strconv.Atoi(msg.Payload)
		r.catchupRegionIfNeeded(v)
	}
}

func (r *refresh) catchupRegionIfNeeded(regionId int) error {
	key := fmt.Sprintf("indexationCatchupLaunch:%d", regionId)
	v, err := r.client.Get(context.Background(), key).Result()

	if (err != nil && err.Error() != "redis: nil") || v == "1" {
		return nil
	}

	now := time.Now().Unix()
	res, _ := r.client.ZRangeByScore(
		context.Background(),
		"indexationDelayed",
		&goredis.ZRangeBy{
			Min: fmt.Sprintf("%d", now),
			Max: fmt.Sprintf("%d", now+600),
		},
	).Result()

	if len(res) == 0 {
		args := goredis.XAddArgs{
			Stream: "indexationCatchup",
			Values: []interface{}{"regionId", regionId},
		}

		log.Infof("Ask to catchup %d", regionId)

		r.client.XAdd(context.Background(), &args)
		r.client.Set(context.Background(), key, true, 5*time.Minute)
	}

	return nil
}
