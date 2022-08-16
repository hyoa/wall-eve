package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/nitishm/go-rejson/v4"
	"github.com/panjf2000/ants/v2"
)

type RedisRepository struct {
	client *goredis.Client
}

func NewRedisRepository(client *goredis.Client) *RedisRepository {
	return &RedisRepository{
		client: client,
	}
}

type OrdersRedis struct {
	IsBuyOrder   bool    `json:"isBuyOrder"`
	RegionId     int32   `json:"regionId"`
	TypeName     string  `json:"typeName,omitempty"`
	RegionName   string  `json:"regionName,omitempty"`
	SystemName   string  `json:"systemName,omitempty"`
	LocationName string  `json:"locationName,omitempty"`
	LocationId   int64   `json:"locationId"`
	Price        float64 `json:"price"`
	SystemId     int64   `json:"systemId"`
	TypeId       int32   `json:"typeId"`
	VolumeTotal  int32   `json:"volumeTotal"`
	IssuedAt     int64   `json:"issuedAt"`
	OrderId      int64   `json:"orderId"`
}

type DenormalizedOrderRedis struct {
	RegionId     int32   `json:"regionId"`
	SystemId     int32   `json:"systemId"`
	LocationId   int64   `json:"locationId"`
	TypeId       int32   `json:"typeId"`
	RegionName   string  `json:"regionName"`
	SystemName   string  `json:"systemName"`
	LocationName string  `json:"locationName"`
	TypeName     string  `json:"typeName"`
	BuyPrice     float64 `json:"buyPrice"`
	SellPrice    float64 `json:"sellPrice"`
	BuyVolume    int32   `json:"buyVolume"`
	SellVolume   int32   `json:"sellVolume"`
}

type OrdersAggregatedRedis struct {
	regionId, typeId int32
	locationId       int64
	price, volume    string
}

type chanSaveOrders struct {
	err bool
}

func (r *RedisRepository) SaveOrders(orders []domain.Order) error {
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClient(r.client)

	pool, _ := ants.NewPoolWithFunc(100, taskSaveOrderFunc, ants.WithPanicHandler(panicHandler))
	defer pool.Release()

	var wg sync.WaitGroup
	wg.Add(len(orders))

	tasks := make([]*taskSaveOrderStruct, 0, len(orders))
	for k := range orders {
		task := &taskSaveOrderStruct{
			wg:    &wg,
			rh:    rh,
			order: orders[k],
			index: k,
		}

		tasks = append(tasks, task)
		pool.Invoke(task)
	}

	wg.Wait()
	countSuccess := 0
	countFail := 0

	for i := range tasks {
		if tasks[i].err {
			countFail++
		} else {
			countSuccess++
		}
	}

	fmt.Println(countSuccess, countFail)

	return nil
}

func (r *RedisRepository) DeleteAllOrdersForRegion(regionId int32) error {
	// ctx := context.Background()
	// log.Infoln("Scanning keys for deletion")
	// iter := r.client.Scan(ctx, 0, fmt.Sprintf("orders:%d:*", regionId), 0).Iterator()

	// log.Infoln("Prepare deletion")
	// pool, _ := ants.NewPoolWithFunc(1000, taskDeleteOrderFunc)
	// defer pool.Release()

	// var wg sync.WaitGroup
	// tasks := make([]*taskDeleteOrderStruct, 0)

	// for iter.Next(ctx) {
	// 	key := iter.Val()

	// 	task := &taskDeleteOrderStruct{
	// 		wg:     &wg,
	// 		client: r.client,
	// 		key:    key,
	// 	}

	// 	wg.Add(1)
	// 	tasks = append(tasks, task)
	// 	pool.Invoke(task)
	// }

	// if err := iter.Err(); err != nil {
	// 	panic(err)
	// }

	// wg.Wait()

	return nil
}

func (r *RedisRepository) DeleteAllOrdersForRegionAndTypeId(regionId, typeId int32) error {
	// ctx := context.Background()
	// log.Infoln("Scanning keys for deletion: ", regionId, typeId)
	// iter := r.client.Scan(ctx, 0, fmt.Sprintf("orders:%d:*", regionId), 0).Iterator()

	// log.Infoln("Prepare deletion")
	// pool, _ := ants.NewPoolWithFunc(100, taskDeleteOrderFunc, ants.WithPanicHandler(panicHandler))
	// defer pool.Release()

	// var wg sync.WaitGroup
	// tasks := make([]*taskDeleteOrderStruct, 0)

	// for iter.Next(ctx) {
	// 	key := iter.Val()

	// 	if strings.Contains(key, fmt.Sprintf("%d", typeId)) {
	// 		task := &taskDeleteOrderStruct{
	// 			wg:     &wg,
	// 			client: r.client,
	// 			key:    key,
	// 		}

	// 		wg.Add(1)
	// 		tasks = append(tasks, task)
	// 		pool.Invoke(task)
	// 	}

	// }

	// if err := iter.Err(); err != nil {
	// 	panic(err)
	// }

	// log.Infoln("Wait for deletion completion")
	// wg.Wait()

	return nil
}

func (r *RedisRepository) AggregateOrdersForRegionAndTypeId(regionId, typeId int32) ([]domain.DenormalizedOrder, error) {
	val, err := r.client.Do(
		context.Background(),
		"FT.AGGREGATE", "ordersIdx", fmt.Sprintf("@typeId:[%d %d] @regionId:[%d, %d]", typeId, typeId, regionId, regionId),
		"GROUPBY", "4", "@typeId", "@isBuyOrder", "@regionId", "@locationId",
		"REDUCE", "MIN", "1", "@price", "AS", "min_price",
		"REDUCE", "MAX", "1", "@price", "AS", "max_price",
		"REDUCE", "SUM", "1", "@volumeTotal", "AS", "volumeTotal",
		"APPLY", "format(\"%s#%s:%s\", @isBuyOrder, @min_price, @max_price)", "AS", "price",
		"APPLY", "format(\"%s#%s\", @isBuyOrder, @volumeTotal)", "AS", "volumeTotal",
		"GROUPBY", "3", "@typeId", "@regionId", "@locationId",
		"REDUCE", "TOLIST", "1", "@price", "AS", "price",
		"REDUCE", "TOLIST", "1", "@volumeTotal", "AS", "volumeTotal",
	).Result()

	if err != nil {
		fmt.Println(err)
	}

	orders := parseAggregateOrders(val)

	denormalizedOrders := make([]domain.DenormalizedOrder, 0, len(orders))

	for k := range orders {
		denormalizedOrder := domain.DenormalizedOrder{
			RegionId:   orders[k].regionId,
			LocationId: orders[k].locationId,
			TypeId:     orders[k].typeId,
		}

		re := regexp.MustCompile(`(?m)(?:0#(?P<sell>\w+):\w+)?,?(?:1#\w+:(?P<buy>\w+))?`)
		groupNames := re.SubexpNames()
		for _, match := range re.FindAllStringSubmatch(orders[k].price, -1) {
			for groupIdx, group := range match {
				name := groupNames[groupIdx]
				if name == "sell" && group != "" {
					v, _ := strconv.ParseFloat(group, 64)
					denormalizedOrder.SellPrice = v
				}

				if name == "buy" && group != "" {
					v, _ := strconv.ParseFloat(group, 64)
					denormalizedOrder.BuyPrice = v
				}
			}

		}

		denormalizedOrders = append(denormalizedOrders, denormalizedOrder)
	}

	return denormalizedOrders, nil
}

func (r *RedisRepository) SaveDenormalizedOrders(orders []domain.DenormalizedOrder) error {
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClient(r.client)

	for k := range orders {
		formattedDenormalizedOrder := DenormalizedOrderRedis{
			RegionId:     orders[k].RegionId,
			RegionName:   orders[k].RegionName,
			LocationId:   orders[k].LocationId,
			LocationName: orders[k].LocationName,
			TypeId:       orders[k].TypeId,
			TypeName:     orders[k].TypeName,
			BuyPrice:     orders[k].BuyPrice,
			SellPrice:    orders[k].SellPrice,
		}

		_, errSet := rh.JSONSet(fmt.Sprintf("denormalizedOrders:%d:%d", formattedDenormalizedOrder.LocationId, formattedDenormalizedOrder.TypeId), ".", formattedDenormalizedOrder)

		if errSet != nil {
			fmt.Println(errSet)
		}
	}

	return nil
}

func (r *RedisRepository) SearchDenormalizedOrders(filter domain.Filter) ([]domain.DenormalizedOrder, error) {
	searchParams := createSearchParams(filter)
	queryParams := fmt.Sprintf(
		"%s @buyPrice:[%.2f %.2f] @sellPrice:[%.2f %.2f]",
		searchParams,
		filter.MinBuyPrice,
		filter.MaxBuyPrice,
		filter.MinSellPrice,
		filter.MaxSellPrice,
	)

	val, err := r.client.Do(
		context.Background(),
		"FT.SEARCH", "denormalizedOrdersIdx",
		queryParams,
		"LIMIT", 0, 10000,
	).Result()

	if err != nil {
		return make([]domain.DenormalizedOrder, 0), err
	}

	ordersParsed := parseSearchOrders(val)

	denormalizedOrders := make([]domain.DenormalizedOrder, 0)
	for k := range ordersParsed {
		denormalizedOrders = append(denormalizedOrders, domain.DenormalizedOrder{
			RegionId:     ordersParsed[k].RegionId,
			SystemId:     ordersParsed[k].SystemId,
			LocationId:   ordersParsed[k].LocationId,
			TypeId:       ordersParsed[k].TypeId,
			RegionName:   ordersParsed[k].RegionName,
			SystemName:   ordersParsed[k].SystemName,
			LocationName: ordersParsed[k].LocationName,
			TypeName:     ordersParsed[k].TypeName,
			BuyPrice:     ordersParsed[k].BuyPrice,
			SellPrice:    ordersParsed[k].SellPrice,
		})
	}

	return denormalizedOrders, nil
}

func createSearchParams(filter domain.Filter) string {
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

func (r *RedisRepository) Get(key string) (string, bool) {
	val, errGet := r.client.Get(context.Background(), key).Result()

	if errGet != nil || val == "" {
		return "", false
	}

	return val, true
}

func (r *RedisRepository) Save(key, value string) error {
	return r.client.Set(context.Background(), key, value, 0).Err()
}

func (r *RedisRepository) SaveOrdersIdFetch(ordersKeys map[domain.KeyOrder][]int64) error {
	pool, _ := ants.NewPoolWithFunc(1000, taskSaveOrdersKeysFunc, ants.WithPanicHandler(panicHandler))
	defer pool.Release()

	var wg sync.WaitGroup
	tasks := make([]*taskSaveOrdersKeysStruct, 0)

	for key := range ordersKeys {
		task := &taskSaveOrdersKeysStruct{
			wg:        &wg,
			client:    r.client,
			key:       key,
			ordersIds: ordersKeys[key],
		}

		wg.Add(1)
		tasks = append(tasks, task)
		pool.Invoke(task)
	}

	wg.Wait()
	return nil
}

func (r *RedisRepository) RemoveOrdersNotInPool(ordersKeys map[domain.KeyOrder][]int64) error {
	pool, _ := ants.NewPoolWithFunc(1000, taskDeleteOrderFunc)
	defer pool.Release()

	var wg sync.WaitGroup
	tasks := make([]*taskDeleteOrderStruct, 0)

	for key := range ordersKeys {
		task := &taskDeleteOrderStruct{
			wg:        &wg,
			client:    r.client,
			key:       key,
			ordersIds: ordersKeys[key],
		}
		wg.Add(1)
		tasks = append(tasks, task)
		pool.Invoke(task)

	}

	wg.Wait()
	return nil
}

func (r *RedisRepository) NotifyReadyToIndex(regionId int32, typeIds map[int32]bool) error {
	pool, _ := ants.NewPoolWithFunc(1000, taskNotifyIndexationFunc, ants.WithPanicHandler(panicHandler))
	defer pool.Release()

	var wg sync.WaitGroup
	tasks := make([]*taskNotifyIndexationStruct, 0)

	for typeId := range typeIds {
		task := &taskNotifyIndexationStruct{
			wg:       &wg,
			client:   r.client,
			typeId:   typeId,
			regionId: regionId,
		}

		wg.Add(1)
		tasks = append(tasks, task)
		pool.Invoke(task)
	}

	wg.Wait()
	return nil
}

func panicHandler(err interface{}) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

type taskSaveOrderStruct struct {
	wg    *sync.WaitGroup
	rh    *rejson.Handler
	order domain.Order
	index int
	err   bool
}

func taskSaveOrderFunc(data interface{}) {
	t := data.(*taskSaveOrderStruct)
	t.saveOrder()
}

func (t *taskSaveOrderStruct) saveOrder() {
	formattedOrder := OrdersRedis{
		IsBuyOrder:   t.order.IsBuyOrder,
		RegionId:     t.order.RegionId,
		TypeName:     t.order.TypeName,
		RegionName:   t.order.TypeName,
		SystemName:   t.order.SystemName,
		LocationName: t.order.LocationName,
		LocationId:   t.order.LocationId,
		Price:        t.order.Price,
		SystemId:     t.order.SystemId,
		TypeId:       t.order.TypeId,
		VolumeTotal:  t.order.VolumeTotal,
		IssuedAt:     t.order.IssuedAt,
		OrderId:      t.order.OrderId,
	}

	res, errSet := t.rh.JSONSet(fmt.Sprintf("orders:%d", formattedOrder.OrderId), ".", formattedOrder)

	if errSet != nil {
		fmt.Println(errSet)
		t.err = true

		t.wg.Done()
	}

	if res.(string) == "OK" {
		t.err = false

		t.wg.Done()
	} else {
		t.err = true

		t.wg.Done()
	}
}

type taskDeleteOrderStruct struct {
	wg        *sync.WaitGroup
	client    *goredis.Client
	key       domain.KeyOrder
	ordersIds []int64
}

func taskDeleteOrderFunc(data interface{}) {
	t := data.(*taskDeleteOrderStruct)
	t.deleteOrder()
}

func (t *taskDeleteOrderStruct) deleteOrder() {
	keyToGet := fmt.Sprintf("ordersKeys:%d:%d", t.key.RegionId, t.key.TypeId)
	ordersIdSaved, _ := t.client.SMembers(context.Background(), keyToGet).Result()

	ordersId := make([]int64, 0)
	for k := range ordersIdSaved {
		v, _ := strconv.Atoi(ordersIdSaved[k])
		ordersId = append(ordersId, int64(v))
	}

	for _, id := range ordersId {
		found := false
		for _, newId := range t.ordersIds {
			if id == int64(newId) {
				found = true
			}
		}

		if !found {
			t.client.Del(context.Background(), fmt.Sprintf("orders:%d", id))
		}
	}

	t.client.Del(context.Background(), keyToGet)
	t.wg.Done()
}

func parseAggregateOrders(data interface{}) []OrdersAggregatedRedis {
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

	orders := make([]OrdersAggregatedRedis, 0)
	for _, el := range elements {
		switch val := el.(type) {
		case []interface{}:
			order := OrdersAggregatedRedis{}
			for k := range val {
				switch val2 := val[k].(type) {
				case string:
					if k == 1 {
						v, _ := strconv.Atoi(val2)
						order.typeId = int32(v)
					}

					if k == 3 {
						v, _ := strconv.Atoi(val2)
						order.regionId = int32(v)
					}

					if k == 5 {
						v, _ := strconv.Atoi(val2)
						order.locationId = int64(v)
					}
				case []interface{}:
					if k == 7 {
						order.price = parseFormattedArray(val2)
					}

					if k == 9 {
						order.volume = parseFormattedArray(val2)
					}
				}
			}
			orders = append(orders, order)
		}
	}

	return orders
}

func parseFormattedArray(data []interface{}) string {
	elements := make([]string, 0)

	for k := range data {
		switch val := data[k].(type) {
		case string:
			elements = append(elements, val)
		}
	}

	return strings.Join(elements, ",")
}

func parseSearchOrders(data interface{}) []DenormalizedOrderRedis {
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

	fmt.Println("-------")
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

	return orders
}

type taskSaveOrdersKeysStruct struct {
	wg        *sync.WaitGroup
	key       domain.KeyOrder
	ordersIds []int64
	client    *goredis.Client
}

func taskSaveOrdersKeysFunc(data interface{}) {
	t := data.(*taskSaveOrdersKeysStruct)
	t.saveKeys()
}

func (t *taskSaveOrdersKeysStruct) saveKeys() {
	keyToSave := fmt.Sprintf("ordersKeys:%d:%d", t.key.RegionId, t.key.TypeId)
	for _, id := range t.ordersIds {
		t.client.SAdd(context.Background(), keyToSave, id).Result()
	}

	t.wg.Done()
}

type taskNotifyIndexationStruct struct {
	wg               *sync.WaitGroup
	client           *goredis.Client
	typeId, regionId int32
}

func taskNotifyIndexationFunc(data interface{}) {
	t := data.(*taskNotifyIndexationStruct)
	t.notifyIndexation()
}

func (t *taskNotifyIndexationStruct) notifyIndexation() {
	args := goredis.XAddArgs{
		Stream: "indexation",
		Values: []interface{}{"regionId", t.regionId, "typeId", t.typeId},
	}

	t.client.XAdd(context.Background(), &args).Result()
	t.wg.Done()
}
