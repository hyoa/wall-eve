package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/denormorder"
)

type OrderController struct {
	client *goredis.Client
}

func NewOrderController(client *goredis.Client) OrderController {
	return OrderController{
		client: client,
	}
}

func (oc *OrderController) GetOrdersWithFilter(ctx *gin.Context) {

	orders, _ := denormorder.GetDenormalizedOrdersWithFilter(createFilter(ctx), oc.client)

	if len(orders) > 0 {
		regionId := orders[0].RegionId
		oc.client.Publish(context.Background(), "denormalizedOrdersRegionSearch", regionId)
	}

	ctx.JSON(http.StatusOK, orders)
}

func createFilter(ctx *gin.Context) denormorder.Filter {
	var filter denormorder.Filter

	if val := ctx.Query("locationName"); val != "" {
		filter.LocationName = val
	}

	if val := ctx.Query("systemName"); val != "" {
		filter.SystemName = val
	}

	if val := ctx.Query("regionName"); val != "" {
		filter.RegionName = val
	}

	if val := ctx.Query("typeName"); val != "" {
		filter.TypeName = val
	}

	if val := ctx.Query("locationId"); val != "" {
		v, _ := strconv.Atoi(val)
		filter.LocationId = v
	}

	if val := ctx.Query("systemId"); val != "" {
		v, _ := strconv.Atoi(val)
		filter.SystemId = v
	}

	if val := ctx.Query("regionId"); val != "" {
		v, _ := strconv.Atoi(val)
		filter.RegionId = v
	}

	if val := ctx.Query("minBuyPrice"); val != "" {
		v, _ := strconv.ParseFloat(val, 64)
		filter.MinBuyPrice = v
	} else {
		filter.MinBuyPrice = 0
	}

	if val := ctx.Query("maxBuyPrice"); val != "" {
		v, _ := strconv.ParseFloat(val, 64)
		filter.MaxBuyPrice = v
	} else {
		filter.MaxBuyPrice = 1000000000000
	}

	if val := ctx.Query("minSellPrice"); val != "" {
		v, _ := strconv.ParseFloat(val, 64)
		filter.MinSellPrice = v
	} else {
		filter.MinSellPrice = 0
	}

	if val := ctx.Query("maxSellPrice"); val != "" {
		v, _ := strconv.ParseFloat(val, 64)
		filter.MaxSellPrice = v
	} else {
		filter.MaxSellPrice = 1000000000000
	}

	return filter
}
