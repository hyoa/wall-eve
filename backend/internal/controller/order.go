package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hyoa/wall-eve/backend/internal/domain"
)

type OrderController struct {
	orderUseCase domain.OrderUseCase
}

func NewOrderController(useCase domain.OrderUseCase) OrderController {
	return OrderController{
		orderUseCase: useCase,
	}
}

func (oc *OrderController) GetOrdersWithFilter(ctx *gin.Context) {

	orders, _ := oc.orderUseCase.GetOrdersWithFilter(createFilter(ctx))

	ctx.JSON(http.StatusOK, orders)
}

func createFilter(ctx *gin.Context) domain.Filter {
	filter := domain.CreateDefaultFilter()

	locationName := ctx.Query("locationName")
	if locationName != "" {
		filter.LocationName = locationName
	}

	regionName := ctx.Query("regionName")
	if regionName != "" {
		filter.RegionName = regionName
	}

	systemName := ctx.Query("systemName")
	if systemName != "" {
		filter.SystemName = systemName
	}

	typeName := ctx.Query("typeName")
	if typeName != "" {
		filter.TypeName = typeName
	}

	regionId := ctx.Query("regionId")
	if regionId != "" {
		v, _ := strconv.Atoi(regionId)
		filter.RegionId = int32(v)
	}

	locationId := ctx.Query("locationId")
	if locationId != "" {
		v, _ := strconv.Atoi(locationId)
		filter.LocationId = int32(v)
	}

	systemId := ctx.Query("systemId")
	if systemId != "" {
		v, _ := strconv.Atoi(systemId)
		filter.SystemId = int32(v)
	}

	minBuyPrice := ctx.Query("minBuyPrice")
	if minBuyPrice != "" {
		v, _ := strconv.ParseFloat(minBuyPrice, 64)
		filter.MinBuyPrice = v
	}

	maxBuyPrice := ctx.Query("maxBuyPrice")
	if maxBuyPrice != "" {
		v, _ := strconv.ParseFloat(maxBuyPrice, 64)
		filter.MaxBuyPrice = v
	}

	minSellPrice := ctx.Query("minSellPrice")
	if minSellPrice != "" {
		v, _ := strconv.ParseFloat(minSellPrice, 64)
		filter.MinSellPrice = v
	}

	maxSellPrice := ctx.Query("maxSellPrice")
	if maxSellPrice != "" {
		v, _ := strconv.ParseFloat(maxSellPrice, 64)
		filter.MaxSellPrice = v
	}

	return filter
}
