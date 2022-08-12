package repository

import (
	"context"
	"fmt"
	"os"
	"sync"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/nitishm/go-rejson/v4"
	"github.com/panjf2000/ants/v2"
	log "github.com/sirupsen/logrus"
)

type RedisRepository struct {
	client *goredis.Client
}

func NewRedisRepository(client *goredis.Client) domain.OrdersRepository {
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
	ctx := context.Background()
	log.Infoln("Scanning keys for deletion")
	iter := r.client.Scan(ctx, 0, fmt.Sprintf("orders:%d:*", regionId), 0).Iterator()

	log.Infoln("Prepare deletion")
	pool, _ := ants.NewPoolWithFunc(100, taskDeleteOrderFunc)
	defer pool.Release()

	var wg sync.WaitGroup
	tasks := make([]*taskDeleteOrderStruct, 0)

	for iter.Next(ctx) {
		key := iter.Val()

		task := &taskDeleteOrderStruct{
			wg:     &wg,
			client: r.client,
			key:    key,
		}

		wg.Add(1)
		tasks = append(tasks, task)
		pool.Invoke(task)
	}

	if err := iter.Err(); err != nil {
		panic(err)
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
	}

	res, errSet := t.rh.JSONSet(fmt.Sprintf("orders:%d:%d:%d", formattedOrder.RegionId, formattedOrder.TypeId, t.index), ".", formattedOrder)

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
	wg     *sync.WaitGroup
	client *goredis.Client
	key    string
}

func taskDeleteOrderFunc(data interface{}) {
	t := data.(*taskDeleteOrderStruct)
	t.deleteOrder()
}

func (t *taskDeleteOrderStruct) deleteOrder() {
	t.client.Del(context.Background(), t.key)
	t.wg.Done()
}
