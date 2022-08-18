package domain

import (
	"fmt"
	"sort"
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
	FetchOrdersForRegionAndType(regionId, typeId int32) ([]RawOrder, error)
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
	AddTotalOrdersForRegionAndType(regionId, typeId, ordersCount int) error
}

type Notifier interface {
	NotifyIndexationFinished(regionId, typeId int) error
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

func (ouc *OrderUseCase) IndexOrdersForRegionAndTypeId(regionId, typeId int32) error {
	log.Infoln("Fetch orders")
	rawOrders, errFetch := ouc.externalOrderRepository.FetchOrdersForRegionAndType(regionId, typeId)

	if errFetch != nil {
		return fmt.Errorf("Unable to fetch orders for %d: %w", regionId, errFetch)
	}

	type keyLocationIdTypeId struct {
		locationId int64
		typeId     int32
	}

	type ordersMappedByLocationAndType struct {
		sellPrices  []float64
		buyPrices   []float64
		sellVolumes []int
		buyVolumes  []int
		systemId    int
		regionId    int
	}

	log.Infoln("Regroup orders")
	ordersMapped := make(map[keyLocationIdTypeId]ordersMappedByLocationAndType)
	for k := range rawOrders {
		var order ordersMappedByLocationAndType
		key := keyLocationIdTypeId{
			typeId:     rawOrders[k].TypeId,
			locationId: rawOrders[k].LocationId,
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
				systemId:    int(rawOrders[k].SystemId),
			}
		}

		if rawOrders[k].IsBuyOrder {
			order.buyPrices = append(order.buyPrices, rawOrders[k].Price)
			order.buyVolumes = append(order.buyVolumes, int(rawOrders[k].VolumeTotal))
		} else {
			order.sellPrices = append(order.sellPrices, rawOrders[k].Price)
			order.sellVolumes = append(order.sellVolumes, int(rawOrders[k].VolumeTotal))
		}

		ordersMapped[key] = order
	}

	log.Infoln("Denormalized orders")
	denormalizedOrders := make([]DenormalizedOrder, 0)
	for k := range ordersMapped {
		sort.Float64s(ordersMapped[k].buyPrices)
		sort.Float64s(ordersMapped[k].sellPrices)
		sort.Ints(ordersMapped[k].buyVolumes)
		sort.Ints(ordersMapped[k].sellVolumes)

		var maxBuyPrice float64
		if len(ordersMapped[k].buyPrices) > 0 {
			maxBuyPrice = ordersMapped[k].buyPrices[len(ordersMapped[k].buyPrices)-1]
		}
		var minSellPrice float64
		if len(ordersMapped[k].sellPrices) > 0 {
			minSellPrice = ordersMapped[k].sellPrices[0]
		}

		denormalizedOrders = append(denormalizedOrders, DenormalizedOrder{
			RegionId:   int32(ordersMapped[k].regionId),
			LocationId: k.locationId,
			SystemId:   int32(ordersMapped[k].systemId),
			TypeId:     k.typeId,
			BuyPrice:   maxBuyPrice,
			SellPrice:  minSellPrice,
		})
	}

	log.Infoln("Fetch denormalizedOrders extra data")
	ordersWithoutStructure := make([]DenormalizedOrder, 0)
	for k := range denormalizedOrders {
		if denormalizedOrders[k].LocationId <= 2147483647 {
			order := denormalizedOrders[k]
			order.LocationName = ouc.externalDataRepository.FetchLocationName(denormalizedOrders[k].LocationId)
			order.RegionName = ouc.externalDataRepository.FetchRegionName(denormalizedOrders[k].RegionId)
			order.SystemName = ouc.externalDataRepository.FetchSystemName(denormalizedOrders[k].SystemId)
			order.TypeName = ouc.externalDataRepository.FetchTypeName(denormalizedOrders[k].TypeId)

			ordersWithoutStructure = append(ordersWithoutStructure, order)
		}
	}

	log.Infoln("Save denormalizedOrders")
	ouc.ordersRepository.SaveDenormalizedOrders(ordersWithoutStructure)
	ouc.notifier.NotifyIndexationFinished(int(regionId), int(typeId))

	return nil
}

func (ouc *OrderUseCase) DeleteIndexedOrdersForRegionAndType(regionId, typeId int) error {
	ouc.ordersRepository.DeleteAllOrdersForRegionAndTypeId(int32(regionId), int32(typeId))

	return nil
}

func (ouc *OrderUseCase) GetOrdersWithFilter(filter Filter) ([]DenormalizedOrder, error) {
	return ouc.ordersRepository.SearchDenormalizedOrders(filter)
}

func parseStringDateToTimestamp(date string) int64 {
	t, _ := time.Parse(time.RFC3339, date)

	return t.Unix()
}
