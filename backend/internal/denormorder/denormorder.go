package denormorder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/nitishm/go-rejson/v4"
	"github.com/panjf2000/ants/v2"
)

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
	RegionId     int
	SystemId     int
	LocationId   int
	MinBuyPrice  float64
	MaxBuyPrice  float64
	MinSellPrice float64
	MaxSellPrice float64
	RegionName   string
	LocationName string
	SystemName   string
	TypeName     string
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

	fmt.Println(queryParams)

	if err != nil {
		return make([]DenormalizedOrder, 0), err
	}

	return parseSearchOrders(val), nil
}

func SaveDenormalizedOrders(regionId int, orders []DenormalizedOrder, client *goredis.Client) error {
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClient(client)

	pool, _ := ants.NewPoolWithFunc(1000, taskSaveDenormalizedOrderHandler)
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
	res, errSet := t.rh.JSONSet(key, ".", t.order)

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
	searchParamsArray := make([]string, 0)

	if filter.LocationId != 0 {
		searchParamsArray = append(searchParamsArray, fmt.Sprintf("@locationId:[%d %d]", filter.LocationId, filter.LocationId))
	}

	if filter.RegionId != 0 {
		searchParamsArray = append(searchParamsArray, fmt.Sprintf("@regionId:[%d %d]", filter.RegionId, filter.RegionId))
	}

	if filter.SystemId != 0 {
		searchParamsArray = append(searchParamsArray, fmt.Sprintf("@systemId:[%d %d]", filter.SystemId, filter.SystemId))
	}

	if filter.LocationName != "" {
		searchParamsArray = append(searchParamsArray, fmt.Sprintf("@locationName:(%s)", filter.LocationName))
	}

	if filter.SystemName != "" {
		searchParamsArray = append(searchParamsArray, fmt.Sprintf("@systemName:(%s)", filter.SystemName))
	}

	if filter.RegionName != "" {
		searchParamsArray = append(searchParamsArray, fmt.Sprintf("@regionName:(%s)", filter.RegionName))
	}

	return strings.Join(searchParamsArray, " ")
}

func parseSearchOrders(data interface{}) []DenormalizedOrder {
	orders := make([]DenormalizedOrder, 0)

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

	fmt.Println("-------")
	for k := range elements {
		switch val := elements[k].(type) {
		case []interface{}:
			for k2 := range val {
				switch val2 := val[k2].(type) {
				case string:
					if val2 != "$" {
						var denormalizedOrderRedis DenormalizedOrder
						json.Unmarshal([]byte(val2), &denormalizedOrderRedis)
						orders = append(orders, denormalizedOrderRedis)
					}
				}
			}
		}
	}

	return orders
}

func RemoveDenormOrdersNotWithTypesIds(regionId int, typesIds []int, client *goredis.Client) error {
	// redisKey := fmt.Sprintf("orders:%d", regionId)
	// typesIdsAggregated, _ := client.SMembers(context.Background(), redisKey).Result()

	// typesToRemove := make([]int, 0)
	// for _, idAggregated := range typesIdsAggregated {
	// 	v, _ := strconv.Atoi(idAggregated)
	// 	found := false

	// 	for _, idOnMarket := range typesIds {
	// 		if v == idOnMarket {
	// 			found = true
	// 			break
	// 		}
	// 	}

	// 	if !found {
	// 		typesToRemove = append(typesToRemove, v)
	// 	}
	// }

	// // client.SRem(context.Background(), redisKey, typesToRemove)
	searchParams := make([]string, 0)
	for _, id := range typesIds {
		searchParams = append(searchParams, fmt.Sprintf("-@typeId:[%d %d]", id, id))
	}

	fmt.Println(searchParams)

	keysToRemove, err := client.Do(
		context.TODO(),
		"FT.SEARCH", "denormalizedOrdersIdx",
		fmt.Sprintf("%s", strings.Join(searchParams, " ")),
	).Result()

	fmt.Println(keysToRemove, err)

	return nil
}
