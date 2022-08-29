package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/extradata"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	client *goredis.Client
}

func Create(client *goredis.Client) Scheduler {
	return Scheduler{
		client: client,
	}
}

func (s *Scheduler) RunScheduleIndexation(callback func()) {
	indexationFinishedlastIdChecked, _ := s.client.Get(context.Background(), "scheduler:indexationFinishedLastId").Result()
	indexationCatchuplastIdChecked, _ := s.client.Get(context.Background(), "scheduler:indexationCatchupLastId").Result()

	if indexationFinishedlastIdChecked == "" {
		indexationFinishedlastIdChecked = "0"
	}

	if indexationCatchuplastIdChecked == "" {
		indexationCatchuplastIdChecked = "0"
	}

	log.Infoln("Listen stream for finished indexation")
	for {
		xReadArgs := goredis.XReadArgs{
			Streams: []string{"indexationFinished", "indexationCatchup", indexationFinishedlastIdChecked, indexationCatchuplastIdChecked},
			Count:   1,
			Block:   2 * time.Second,
		}
		res, _ := s.client.XRead(context.Background(), &xReadArgs).Result()

		if len(res) > 0 && len(res[0].Messages) > 0 {
			stream := res[0]
			messages := stream.Messages

			var regionId int
			for k := range messages[0].Values {
				if k == "regionId" {
					switch val := messages[0].Values[k].(type) {
					case string:
						v, _ := strconv.Atoi(val)
						regionId = v
					}
				}
			}

			switch stream.Stream {
			case "indexationFinished":
				log.Infof("Schedule region %d", regionId)
				s.scheduleOrdersScanForRegion(regionId)
				indexationFinishedlastIdChecked = stream.Messages[0].ID
				s.client.Set(context.Background(), "scheduler:indexationFinishedLastId", indexationFinishedlastIdChecked, 0)
				break
			case "indexationCatchup":
				log.Infof("Catchup region %d", regionId)
				s.scheduleOrdersCatchupForRegion(regionId)
				indexationCatchuplastIdChecked = stream.Messages[0].ID
				s.client.Set(context.Background(), "scheduler:indexationCatchupLastId", indexationCatchuplastIdChecked, 0)
				break
			}

		}

		defer callback()
	}
}

func (s *Scheduler) scheduleOrdersScanForRegion(regionId int) error {
	if regionId == 0 || !doesRegionExist(regionId, s.client) {
		log.Errorln("Invalid region")
		return nil
	}

	delay := 3600
	if isRegionSearchDuringInterval(regionId, int(time.Now().Add(-5*time.Minute).UnixMilli()), 300000, s.client) {
		delay = 300
	} else if isRegionSearchDuringInterval(regionId, int(time.Now().Add(-1*time.Hour).UnixMilli()), 3600000, s.client) {
		delay = 600
	}

	delayedTime := int(time.Now().Unix()) + delay

	s.client.ZAdd(context.Background(), "indexationDelayed", &goredis.Z{Score: float64(delayedTime), Member: regionId})

	return nil
}

func isRegionSearchDuringInterval(regionId int, timeStart int, timeWindow int, client *goredis.Client) bool {
	res, _ := client.Do(
		context.Background(),
		"TS.RANGE", fmt.Sprintf("regionFetchHistory:%d", regionId), timeStart, "+", "AGGREGATION", "sum", timeWindow,
	).Result()

	count := 0
	switch val := res.(type) {
	case []interface{}:
		for _, el := range val {
			switch val2 := el.(type) {
			case []interface{}:
				for _, el2 := range val2 {
					switch val3 := el2.(type) {
					case string:
						v, _ := strconv.Atoi(val3)
						count = v
					}
				}
			}
		}
	}

	fmt.Println(res)
	return count > 0
}

func (s *Scheduler) scheduleOrdersCatchupForRegion(regionId int) error {
	if regionId == 0 || !doesRegionExist(regionId, s.client) {
		log.Errorln("Invalid region")
		return nil
	}

	xAddArgs := goredis.XAddArgs{
		Stream: "indexationAdd",
		Values: []interface{}{"regionId", regionId},
	}
	s.client.XAdd(context.Background(), &xAddArgs)

	return nil
}

func doesRegionExist(regionId int, client *goredis.Client) bool {
	valValid, _ := client.SIsMember(context.Background(), "validRegions", regionId).Result()

	if valValid {
		return true
	}

	valInvalid, errGetFromRedis := client.SIsMember(context.Background(), "invalidRegions", regionId).Result()

	if valInvalid {
		return false
	}

	if !valInvalid || errGetFromRedis != nil {
		name, errGetFromEsi := extradata.GetRegionName(regionId)

		if name == "" || errGetFromEsi != nil {
			client.SAdd(context.Background(), "invalidRegions", regionId)

			return false
		}
	}

	client.SAdd(context.Background(), "validRegions", regionId)
	return true
}
