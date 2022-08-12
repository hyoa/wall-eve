package repository

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/hyoa/wall-eve/backend/internal/domain"
)

type chanFetchOrders struct {
	orders []domain.RawOrder
	url    string
	err    bool
}

type EsiRepository struct{}

func (er *EsiRepository) FetchOrders(regionId int32) ([]domain.RawOrder, error) {
	return getOrdersForRegion(int(regionId))
}

func getNbPages(url string) int {
	resp, err := http.Head(url)

	if err != nil {
		fmt.Println("Unable to fetch head: ", url)
	}

	nbPages, _ := strconv.ParseInt(resp.Header.Get("X-Pages"), 10, 32)

	return int(nbPages)
}

func getOrdersForRegion(r int) ([]domain.RawOrder, error) {
	o := make([]domain.RawOrder, 0)
	// headUrl := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/orders/?datasource=tranquility&order_type=all&page=1", r)
	// nbPages := getNbPages(headUrl)
	nbPages := 1

	c := make(chan chanFetchOrders)

	for p := 1; p <= nbPages; p++ {
		go getOrdersForRegionAndPage(r, p, c)
	}

	for i := 1; i <= nbPages; i++ {
		resp := <-c
		o = append(o, resp.orders...)
	}

	return o, nil
}

func getOrdersForRegionAndPage(r int, p int, c chan chanFetchOrders) {
	u := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/orders/?datasource=tranquility&order_type=all&page=%d", r, p)

	resp, err := http.Get(u)

	if err != nil {
		fmt.Printf("Unable to fetch orders for url %s", u)
		c <- chanFetchOrders{err: true}
	}

	b, errBody := ioutil.ReadAll(resp.Body)

	if errBody != nil {
		fmt.Printf("Unable to fetch orders for url %s", u)
		c <- chanFetchOrders{err: true}
	}

	var orders []domain.RawOrder
	json.Unmarshal(b, &orders)

	c <- chanFetchOrders{orders: orders, url: u, err: false}
}
