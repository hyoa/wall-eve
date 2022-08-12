package domain

import (
	"fmt"
	"time"
)

type OrderUseCase struct {
	externalOrderRepository ExternalOrdersRepository
	ordersRepository        OrdersRepository
}

type RawOrder struct {
	IsBuyOrder  bool    `json:"is_buy_order"`
	LocationId  int64   `json:"location_id"`
	Price       float64 `json:"price"`
	SystemId    int64   `json:"system_id"`
	TypeId      int32   `json:"type_id"`
	VolumeTotal int32   `json:"volume_total"`
	IssuedAt    string  `json:"issued"`
}

type Order struct {
	IsBuyOrder   bool    `json:"isBuyOrder"`
	RegionId     int32   `json:"regionId"`
	TypeName     string  `json:"typeName"`
	RegionName   string  `json:"regionName"`
	SystemName   string  `json:"systemName"`
	LocationName string  `json:"locationName"`
	LocationId   int64   `json:"locationId"`
	Price        float64 `json:"price"`
	SystemId     int64   `json:"systemId"`
	TypeId       int32   `json:"typeId"`
	VolumeTotal  int32   `json:"volumeTotal"`
	IssuedAt     int64   `json:"issuedAt"`
}

type ExternalOrdersRepository interface {
	FetchOrders(regionId int32) ([]RawOrder, error)
}

type OrdersRepository interface {
	SaveOrders([]Order) error
	DeleteAllOrdersForRegion(regionId int32) error
}

func NewOrderUseCase(externalOrderRepository ExternalOrdersRepository, ordersRepository OrdersRepository) OrderUseCase {
	return OrderUseCase{
		externalOrderRepository: externalOrderRepository,
		ordersRepository:        ordersRepository,
	}
}

func (ouc *OrderUseCase) FetchAllOrdersForRegion(regionId int32) error {
	rawOrders, errFetch := ouc.externalOrderRepository.FetchOrders(regionId)

	if errFetch != nil {
		return fmt.Errorf("Unable to fetch orders for %d: %w", regionId, errFetch)
	}

	orders := make([]Order, 0)
	for k := range rawOrders {
		orders = append(orders, Order{
			IsBuyOrder:   rawOrders[k].IsBuyOrder,
			RegionId:     regionId,
			TypeName:     "",
			RegionName:   "",
			SystemName:   "",
			LocationName: "",
			LocationId:   rawOrders[k].LocationId,
			Price:        rawOrders[k].Price,
			SystemId:     rawOrders[k].SystemId,
			TypeId:       rawOrders[k].TypeId,
			VolumeTotal:  rawOrders[k].VolumeTotal,
			IssuedAt:     parseStringDateToTimestamp(rawOrders[k].IssuedAt),
		})
	}

	errDelete := ouc.ordersRepository.DeleteAllOrdersForRegion(regionId)
	if errDelete != nil {
		return fmt.Errorf("Unable to delete orders for %d: %w", regionId, errFetch)
	}

	errSave := ouc.ordersRepository.SaveOrders(orders)
	if errSave != nil {
		return fmt.Errorf("Unable to save orders for %d: %w", regionId, errFetch)
	}

	return nil
}

func parseStringDateToTimestamp(date string) int64 {
	t, _ := time.Parse(time.RFC3339, date)

	return t.Unix()
}
