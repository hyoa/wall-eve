package denormorder

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/nitishm/go-rejson/v4"
	"github.com/panjf2000/ants/v2"
)

type DenormalizedOrderRedis struct {
	RegionId       int     `json:"regionId"`
	SystemId       int     `json:"systemId"`
	LocationId     int     `json:"locationId"`
	TypeId         int     `json:"typeId"`
	RegionName     string  `json:"regionName"`
	SystemName     string  `json:"systemName"`
	LocationName   string  `json:"locationName"`
	TypeName       string  `json:"typeName"`
	BuyPrice       float64 `json:"buyPrice"`
	SellPrice      float64 `json:"sellPrice"`
	BuyVolume      int     `json:"buyVolume"`
	SellVolume     int     `json:"sellVolume"`
	LocationIdTags string  `json:"locationIdTags"`
}

type DenormalizedOrder struct {
	RegionId     int     `json:"regionId"`
	SystemId     int     `json:"systemId"`
	LocationId   int     `json:"locationId"`
	TypeId       int     `json:"typeId"`
	RegionName   string  `json:"regionName"`
	SystemName   string  `json:"systemName"`
	LocationName string  `json:"locationName"`
	TypeName     string  `json:"typeName"`
	BuyPrice     float64 `json:"buyPrice"`
	SellPrice    float64 `json:"sellPrice"`
	BuyVolume    int     `json:"buyVolume"`
	SellVolume   int     `json:"sellVolume"`
}

type Filter struct {
	MinBuyPrice  float64
	MaxBuyPrice  float64
	MinSellPrice float64
	MaxSellPrice float64
	TypeName     string
	Location     string
}

func GetDenormalizedOrdersWithFilter(filter Filter, client *goredis.Client) ([]DenormalizedOrder, error) {
	searchParams := createSearchParams(filter)
	queryParams := fmt.Sprintf(
		"%s @buyPrice:[%.2f %.2f] @sellPrice:[%.2f %.2f]",
		searchParams,
		filter.MinBuyPrice,
		filter.MaxBuyPrice,
		filter.MinSellPrice,
		filter.MaxSellPrice,
	)

	val, err := client.Do(
		context.Background(),
		"FT.SEARCH", "denormalizedOrdersIdx",
		queryParams,
		"LIMIT", 0, 10000,
	).Result()

	if err != nil {
		return make([]DenormalizedOrder, 0), err
	}

	return parseSearchOrders(val), nil
}

func SaveDenormalizedOrders(regionId int, orders []DenormalizedOrder, client *goredis.Client) error {
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClient(client)

	pool, _ := ants.NewPoolWithFunc(100, taskSaveDenormalizedOrderHandler)
	defer pool.Release()

	var wg sync.WaitGroup
	tasks := make([]*taskSaveDenormalizedOrderPayload, 0)

	for k := range orders {
		wg.Add(1)

		task := &taskSaveDenormalizedOrderPayload{
			wg:     &wg,
			order:  orders[k],
			err:    false,
			rh:     rh,
			client: client,
		}

		tasks = append(tasks, task)
		pool.Invoke(task)
	}

	wg.Wait()

	keys := make([]string, 0)
	for _, task := range tasks {
		keys = append(keys, task.key)
	}

	return nil
}

func taskSaveDenormalizedOrderHandler(data interface{}) {
	t := data.(*taskSaveDenormalizedOrderPayload)
	t.save()
}

type taskSaveDenormalizedOrderPayload struct {
	wg     *sync.WaitGroup
	rh     *rejson.Handler
	client *goredis.Client
	order  DenormalizedOrder
	err    bool
	key    string
}

func (t *taskSaveDenormalizedOrderPayload) save() {
	key := fmt.Sprintf("denormalizedOrders:%d:%d", t.order.LocationId, t.order.TypeId)

	denormOrderRedis := DenormalizedOrderRedis{
		RegionId:       t.order.RegionId,
		SystemId:       t.order.SystemId,
		LocationId:     t.order.LocationId,
		TypeId:         t.order.TypeId,
		RegionName:     t.order.RegionName,
		SystemName:     t.order.SystemName,
		LocationName:   t.order.LocationName,
		TypeName:       t.order.TypeName,
		BuyPrice:       t.order.BuyPrice,
		SellPrice:      t.order.SellPrice,
		BuyVolume:      t.order.BuyVolume,
		SellVolume:     t.order.SellVolume,
		LocationIdTags: fmt.Sprintf("%d, %d, %d", t.order.RegionId, t.order.SystemId, t.order.LocationId),
	}

	res, errSet := t.rh.JSONSet(key, ".", denormOrderRedis)

	if errSet != nil || res.(string) != "OK" {
		t.err = true
	} else {
		t.client.Expire(context.Background(), key, 24*time.Hour)
		t.err = false
		t.key = key
	}

	t.wg.Done()
}

func createSearchParams(filter Filter) string {
	if locationInt, err := strconv.Atoi(filter.Location); err == nil {
		return fmt.Sprintf("@locationIdTags:{%d}", locationInt)
	}

	return fmt.Sprintf("@locationName:(%s)|@systemName:(%s)|@regionName:(%s)", filter.Location, filter.Location, filter.Location)
}

func parseSearchOrders(data interface{}) []DenormalizedOrder {
	orders := make([]DenormalizedOrderRedis, 0)

	elements := make([]interface{}, 0)

	// Remove counter
	switch val := data.(type) {
	case []interface{}:
		for i := 1; i < len(val); i++ {
			elements = append(elements, val[i])
		}
	default:
		panic("Wrong element")
	}

	for k := range elements {
		switch val := elements[k].(type) {
		case []interface{}:
			for k2 := range val {
				switch val2 := val[k2].(type) {
				case string:
					if val2 != "$" {
						var denormalizedOrderRedis DenormalizedOrderRedis
						json.Unmarshal([]byte(val2), &denormalizedOrderRedis)
						orders = append(orders, denormalizedOrderRedis)
					}
				}
			}
		}
	}

	denormOrders := make([]DenormalizedOrder, 0)
	for k := range orders {
		denormOrders = append(denormOrders, DenormalizedOrder{
			RegionId:     orders[k].RegionId,
			SystemId:     orders[k].SystemId,
			LocationId:   orders[k].LocationId,
			TypeId:       orders[k].TypeId,
			RegionName:   orders[k].RegionName,
			SystemName:   orders[k].SystemName,
			LocationName: orders[k].LocationName,
			TypeName:     orders[k].TypeName,
			BuyPrice:     orders[k].BuyPrice,
			SellPrice:    orders[k].SellPrice,
			BuyVolume:    orders[k].BuyVolume,
			SellVolume:   orders[k].SellVolume,
		})
	}

	return denormOrders
}
