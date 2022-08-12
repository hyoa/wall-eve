package repository

import (
	"context"
	"fmt"

	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/domain"
	"github.com/nitishm/go-rejson/v4"
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
	c := make(chan chanSaveOrders)

	for k := range orders {
		go saveOrder(orders[k], rh, k, c)
	}

	countSuccess := 0
	countFail := 0
	for i := 0; i < len(orders); i++ {
		resp := <-c

		if resp.err {
			countFail++
		} else {
			countSuccess++
		}
	}

	fmt.Println(countSuccess, countFail)

	return nil
}

func saveOrder(order domain.Order, rh *rejson.Handler, index int, c chan chanSaveOrders) {
	formattedOrder := OrdersRedis{
		IsBuyOrder:   order.IsBuyOrder,
		RegionId:     order.RegionId,
		TypeName:     order.TypeName,
		RegionName:   order.TypeName,
		SystemName:   order.SystemName,
		LocationName: order.LocationName,
		LocationId:   order.LocationId,
		Price:        order.Price,
		SystemId:     order.SystemId,
		TypeId:       order.TypeId,
		VolumeTotal:  order.VolumeTotal,
		IssuedAt:     order.IssuedAt,
	}

	res, errSet := rh.JSONSet(fmt.Sprintf("orders:%d:%d:%d", formattedOrder.RegionId, formattedOrder.TypeId, index), ".", formattedOrder)

	if errSet != nil {
		fmt.Println(errSet)
		c <- chanSaveOrders{err: true}
	}

	if res.(string) == "OK" {
		c <- chanSaveOrders{err: false}
	} else {
		c <- chanSaveOrders{err: true}
	}

}

func (r *RedisRepository) DeleteAllOrdersForRegion(regionId int32) error {
	ctx := context.Background()
	iter := r.client.Scan(ctx, 0, fmt.Sprintf("orders:%d:*", regionId), 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		r.client.Del(ctx, key)
	}
	if err := iter.Err(); err != nil {
		panic(err)
	}

	return nil
}
