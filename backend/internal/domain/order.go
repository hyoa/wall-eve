package domain

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type OrderUseCase struct {
	externalOrderRepository ExternalOrdersRepository
	ordersRepository        OrdersRepository
	externalDataRepository  ExternalDataRepository
	notifier                Notifier
}

type RawOrder struct {
	IsBuyOrder  bool    `json:"is_buy_order"`
	LocationId  int64   `json:"location_id"`
	Price       float64 `json:"price"`
	SystemId    int64   `json:"system_id"`
	TypeId      int32   `json:"type_id"`
	VolumeTotal int32   `json:"volume_total"`
	IssuedAt    string  `json:"issued"`
	OrderId     int64   `json:"order_id"`
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
	OrderId      int64   `json:"orderId"`
}

type DenormalizedOrder struct {
	RegionId     int32   `json:"region_id"`
	SystemId     int32   `json:"system_id"`
	LocationId   int64   `json:"location_id"`
	TypeId       int32   `json:"type_id"`
	RegionName   string  `json:"region_name"`
	SystemName   string  `json:"system_name"`
	LocationName string  `json:"location_name"`
	TypeName     string  `json:"type_name"`
	BuyPrice     float64 `json:"buy_price"`
	SellPrice    float64 `json:"sell_price"`
	BuyVolume    int32   `json:"buy_volume"`
	SellVolume   int32   `json:"sell_volume"`
}

type ExternalDataRepository interface {
	FetchTypeName(typeId int32) string
	FetchRegionName(regionId int32) string
	FetchSystemName(systemId int32) string
	FetchLocationName(LocationId int64) string
}

type ExternalOrdersRepository interface {
	FetchOrders(regionId int32) ([]RawOrder, error)
}

type KeyOrder struct {
	RegionId, TypeId int32
}

type OrdersRepository interface {
	SaveOrders([]Order) error
	DeleteAllOrdersForRegion(regionId int32) error
	DeleteAllOrdersForRegionAndTypeId(regionId, typeId int32) error
	AggregateOrdersForRegionAndTypeId(regionId int32, typeId int32) ([]DenormalizedOrder, error)
	SaveDenormalizedOrders([]DenormalizedOrder) error
	SearchDenormalizedOrders(filter Filter) ([]DenormalizedOrder, error)
	SaveOrdersIdFetch(ordersKeys map[KeyOrder][]int64) error
	RemoveOrdersNotInPool(ordersKeys map[KeyOrder][]int64) error
}

type Notifier interface {
	NotifyReadyToIndex(regionId int32, typeIds map[int32]bool) error
}

type Filter struct {
	RegionId     int32
	SystemId     int32
	LocationId   int32
	MinBuyPrice  float64
	MaxBuyPrice  float64
	MinSellPrice float64
	MaxSellPrice float64
	RegionName   string
	LocationName string
	SystemName   string
	TypeName     string
}

func CreateDefaultFilter() Filter {
	return Filter{
		MinBuyPrice:  0,
		MaxBuyPrice:  1000000000000,
		MinSellPrice: 0,
		MaxSellPrice: 10000000000000,
	}
}

func NewOrderUseCase(externalOrderRepository ExternalOrdersRepository, ordersRepository OrdersRepository, externalDataRepository ExternalDataRepository, notifier Notifier) OrderUseCase {
	return OrderUseCase{
		externalOrderRepository: externalOrderRepository,
		ordersRepository:        ordersRepository,
		externalDataRepository:  externalDataRepository,
		notifier:                notifier,
	}
}

func (ouc *OrderUseCase) FetchAllOrdersForRegion(regionId int32) error {
	log.Infoln("Fetch orders")
	rawOrders, errFetch := ouc.externalOrderRepository.FetchOrders(regionId)

	if errFetch != nil {
		return fmt.Errorf("Unable to fetch orders for %d: %w", regionId, errFetch)
	}

	ordersMapped := make(map[KeyOrder][]int64)
	typeToIndex := make(map[int32]bool)

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
			OrderId:      rawOrders[k].OrderId,
		})

		key := KeyOrder{
			RegionId: regionId,
			TypeId:   rawOrders[k].TypeId,
		}

		ordersMapped[key] = append(ordersMapped[key], rawOrders[k].OrderId)
		typeToIndex[rawOrders[k].TypeId] = true
	}

	log.Infoln("Start deletion")
	ouc.ordersRepository.RemoveOrdersNotInPool(ordersMapped)
	log.Infoln("Deletion completed")

	log.Infoln("Start save order data")
	errSave := ouc.ordersRepository.SaveOrders(orders)
	if errSave != nil {
		return fmt.Errorf("Unable to save orders for %d: %w", regionId, errSave)
	}
	log.Infoln("Save orders data completed")

	log.Infoln("Start save orders keys")
	ouc.ordersRepository.SaveOrdersIdFetch(ordersMapped)
	log.Infoln("Save orders keys completed")

	log.Infoln("Notify ready to index")
	ouc.notifier.NotifyReadyToIndex(regionId, typeToIndex)
	log.Infoln("End notification")

	return nil
}

func (ouc *OrderUseCase) IndexOrdersForRegionAndTypeId(regionId, typeId int32) error {
	orders, errAggregate := ouc.ordersRepository.AggregateOrdersForRegionAndTypeId(regionId, typeId)

	if errAggregate != nil {
		return fmt.Errorf("Unable to aggregate orders: %w", errAggregate)
	}

	ordersWithoutStructure := make([]DenormalizedOrder, 0)
	for k := range orders {
		if orders[k].LocationId <= 2147483647 {
			order := orders[k]
			order.LocationName = ouc.externalDataRepository.FetchLocationName(orders[k].LocationId)
			order.RegionName = ouc.externalDataRepository.FetchRegionName(orders[k].RegionId)
			// order.SystemName = ouc.externalDataRepository.FetchSystemName(typeId)
			order.TypeName = ouc.externalDataRepository.FetchTypeName(orders[k].TypeId)

			ordersWithoutStructure = append(ordersWithoutStructure, order)
		}
	}

	ouc.ordersRepository.SaveDenormalizedOrders(ordersWithoutStructure)

	return nil
}

func (ouc *OrderUseCase) GetOrdersWithFilter(filter Filter) ([]DenormalizedOrder, error) {
	return ouc.ordersRepository.SearchDenormalizedOrders(filter)
}

func parseStringDateToTimestamp(date string) int64 {
	t, _ := time.Parse(time.RFC3339, date)

	return t.Unix()
}
