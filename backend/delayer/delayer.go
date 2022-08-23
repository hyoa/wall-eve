package delayer

import (
	"context"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type Delayer struct {
	client *goredis.Client
}

func Create(client *goredis.Client) Delayer {
	return Delayer{
		client: client,
	}
}

func (d *Delayer) Run() {
	log.Infoln("Read tasks queue to send indexation")
	for {
		res, _ := d.client.ZRangeWithScores(context.Background(), "indexationDelayed", 0, 0).Result()

		if len(res) > 0 && res[0].Score <= float64(time.Now().Unix()) {
			var regionId int

			switch val := res[0].Member.(type) {
			case string:
				regionId, _ = strconv.Atoi(val)
			}

			if regionId != 0 {
				log.Infoln("Found 1 item to index")
				xAddArgs := goredis.XAddArgs{
					Stream: "indexationAdd",
					Values: []interface{}{"regionId", regionId},
				}
				d.client.XAdd(context.Background(), &xAddArgs)
			}

			d.client.ZRem(context.Background(), "indexationDelayed", res[0].Member)
		}

		time.Sleep(200 * time.Millisecond)

		defer func() {
			if err := d.client.Close(); err != nil {
				log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
			}
		}()
	}
}
