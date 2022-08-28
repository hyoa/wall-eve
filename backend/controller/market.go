package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"github.com/hyoa/wall-eve/backend/internal/denormorder"
)

type MarketController struct {
	client *goredis.Client
}

func NewOrderController(client *goredis.Client) MarketController {
	return MarketController{
		client: client,
	}
}

func (mc *MarketController) GetDenormOrdersWithFilter(ctx *gin.Context) {

	filter, errFilter := createFilter(ctx)

	if errFilter != nil {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": errFilter.Error()})
	}

	orders, _ := denormorder.GetDenormalizedOrdersWithFilter(filter, mc.client)

	if len(orders) > 0 {
		regionId := orders[0].RegionId
		mc.client.Publish(context.Background(), "apiEvent", regionId)
	}

	ctx.JSON(http.StatusOK, orders)
}

func createFilter(ctx *gin.Context) (denormorder.Filter, error) {
	var filter denormorder.Filter

	if val := ctx.Query("location"); val != "" {
		filter.Location = strings.ReplaceAll(val, "-", "")
	} else {
		return denormorder.Filter{}, errors.New("query parameter location is mandatory")
	}

	if val := ctx.Query("typeName"); val != "" {
		filter.TypeName = val
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

	return filter, nil
}
