package indexer

import (
	"context"
	"sort"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/denormorder"
	"github.com/hyoa/wall-eve/backend/internal/extradata"
	"github.com/hyoa/wall-eve/backend/internal/order"
	log "github.com/sirupsen/logrus"
)

type Indexer struct {
	client *goredis.Client
}

func Create(client *goredis.Client) Indexer {
	return Indexer{
		client: client,
	}
}

func (i *Indexer) Run(consumerName string) {
	log.Infoln("Read indexation stream to launch indexation")
	checkBackLog := true

	for {

		var idToCheck string
		if checkBackLog {
			idToCheck = "0"
		} else {
			idToCheck = ">"
		}

		xAddArgs := goredis.XReadGroupArgs{
			Streams:  []string{"indexationAdd", idToCheck},
			Count:    1,
			Block:    2 * time.Second,
			Group:    "indexationAddGroup",
			Consumer: consumerName,
		}

		res, _ := i.client.XReadGroup(context.Background(), &xAddArgs).Result()

		type streamItem struct {
			id     string
			values map[string]string
		}

		if len(res) > 0 && len(res[0].Messages) == 1 {
			stream := res[0]
			messages := stream.Messages
			if checkBackLog && len(messages) == 0 {
				checkBackLog = false
			}

			regionId := parseMessagePayload(messages[0].Values)

			if regionId != 0 {
				i.indexOrdersInRegion(regionId)
				i.notifyEndOfIndexation(regionId)
			}

			_, errAck := i.client.XAck(context.Background(), "indexationAdd", "indexationAddGroup", messages[0].ID).Result()

			if errAck != nil {
				log.Errorln(errAck)
			}

			_, errDel := i.client.XDel(context.Background(), "indexationAdd", messages[0].ID).Result()

			if errDel != nil {
				log.Errorln(errDel)
			}
		} else if checkBackLog && len(res) == 0 {
			checkBackLog = false
		} else if len(res) > 0 && len(res[0].Messages) == 0 {
			checkBackLog = false
		}

		defer func() {
			if err := i.client.Close(); err != nil {
				log.Fatalf("goredis - failed to communicate to redis-server: %v", err)
			}
		}()
	}
}

func (i *Indexer) indexOrdersInRegion(regionId int) (int, int, error) {
	start := time.Now()
	log.Infoln("Fetch orders")
	orders := order.GetOrdersFromEsiForRegion(regionId)

	type keyLocationIdTypeId struct {
		locationId int
		typeId     int
	}

	type ordersMappedByLocationAndType struct {
		sellPrices  []float64
		buyPrices   []float64
		sellVolumes []int
		buyVolumes  []int
		systemId    int
		regionId    int
	}

	ordersMapped := make(map[keyLocationIdTypeId]ordersMappedByLocationAndType)
	extraData := make(map[string]map[int]string)

	extraData["stations"] = make(map[int]string)
	extraData["systems"] = make(map[int]string)
	extraData["regions"] = make(map[int]string)
	extraData["types"] = make(map[int]string)

	log.Infof("Group %d orders by location and type", len(orders))

	for k := range orders {
		// We avoid players structure, as it require an authentication to get some data.
		// Not hard to do, but out of the scope :)
		if orders[k].LocationId > 2147483647 {
			continue
		}

		var order ordersMappedByLocationAndType
		key := keyLocationIdTypeId{
			typeId:     orders[k].TypeId,
			locationId: orders[k].LocationId,
		}

		if val, ok := ordersMapped[key]; ok {
			order = ordersMappedByLocationAndType{
				sellPrices:  val.sellPrices,
				buyPrices:   val.buyPrices,
				sellVolumes: val.sellVolumes,
				buyVolumes:  val.buyVolumes,
				regionId:    val.regionId,
				systemId:    val.systemId,
			}
		} else {
			order = ordersMappedByLocationAndType{
				sellPrices:  make([]float64, 0),
				buyPrices:   make([]float64, 0),
				sellVolumes: make([]int, 0),
				buyVolumes:  make([]int, 0),
				regionId:    int(regionId),
				systemId:    int(orders[k].SystemId),
			}
		}

		if orders[k].IsBuyOrder {
			order.buyPrices = append(order.buyPrices, orders[k].Price)
			order.buyVolumes = append(order.buyVolumes, int(orders[k].VolumeTotal))
		} else {
			order.sellPrices = append(order.sellPrices, orders[k].Price)
			order.sellVolumes = append(order.sellVolumes, int(orders[k].VolumeTotal))
		}

		extraData["stations"][int(orders[k].LocationId)] = ""
		extraData["systems"][int(orders[k].SystemId)] = ""
		extraData["regions"][int(regionId)] = ""
		extraData["types"][int(orders[k].TypeId)] = ""
		ordersMapped[key] = order
	}

	log.Infoln("Fetch denormalizedOrders extra data")
	extraDataWithName := extradata.FetchExtraData(extraData, i.client)

	log.Infof("Denormalized orders %d", len(ordersMapped))
	denormalizedOrders := make([]denormorder.DenormalizedOrder, 0)
	for k := range ordersMapped {
		sort.Float64s(ordersMapped[k].buyPrices)
		sort.Float64s(ordersMapped[k].sellPrices)

		var maxBuyPrice float64
		if len(ordersMapped[k].buyPrices) > 0 {
			maxBuyPrice = ordersMapped[k].buyPrices[len(ordersMapped[k].buyPrices)-1]
		}
		var minSellPrice float64
		if len(ordersMapped[k].sellPrices) > 0 {
			minSellPrice = ordersMapped[k].sellPrices[0]
		}

		totalBuyVolume := 0
		for _, n := range ordersMapped[k].buyVolumes {
			totalBuyVolume += n
		}

		totalSellVolume := 0
		for _, n := range ordersMapped[k].sellVolumes {
			totalSellVolume += n
		}

		denormalizedOrders = append(denormalizedOrders, denormorder.DenormalizedOrder{
			RegionId:     ordersMapped[k].regionId,
			LocationId:   k.locationId,
			SystemId:     ordersMapped[k].systemId,
			TypeId:       k.typeId,
			BuyPrice:     maxBuyPrice,
			SellPrice:    minSellPrice,
			LocationName: extraDataWithName["stations"][int(k.locationId)],
			SystemName:   extraDataWithName["systems"][ordersMapped[k].systemId],
			RegionName:   extraDataWithName["regions"][ordersMapped[k].regionId],
			TypeName:     extraDataWithName["types"][int(k.typeId)],
			BuyVolume:    totalBuyVolume,
			SellVolume:   totalSellVolume,
		})
	}

	log.Infof("Save denormalizedOrders %d", len(denormalizedOrders))
	denormorder.SaveDenormalizedOrders(regionId, denormalizedOrders, i.client)

	elapsed := time.Since(start)
	log.Infof("Indexation end in: %.f seconds", elapsed.Seconds())
	return 0, 0, nil
}

func (i *Indexer) notifyEndOfIndexation(regionId int) {
	args := goredis.XAddArgs{
		Stream: "indexationFinished",
		Values: []interface{}{"regionId", regionId},
	}

	i.client.XAdd(context.Background(), &args).Result()
}

func parseMessagePayload(values map[string]interface{}) int {
	var regionId int

	for k := range values {
		switch val := values[k].(type) {
		case string:
			if k == "regionId" {
				v, _ := strconv.Atoi(val)
				regionId = v
			}
		}
	}

	return regionId
}
