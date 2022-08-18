package repository

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/hyoa/wall-eve/backend/internal/domain"
	log "github.com/sirupsen/logrus"
)

type chanFetchOrders struct {
	orders []domain.RawOrder
	url    string
	err    bool
}

type chanFetchItemsOnMarket struct {
	ids []int
	url string
	err bool
}

type EsiRepository struct{}

func (er *EsiRepository) FetchOrdersForRegionAndType(regionId, typeId int32) ([]domain.RawOrder, error) {
	return getOrdersForRegionAndType(int(regionId), int(typeId))
}

type UniverseElementName struct {
	Name string `json:"name"`
}

func (er *EsiRepository) FetchElementName(typeId int64, kind string) string {
	url := fmt.Sprintf("https://esi.evetech.net/latest/universe/%s/%d/?datasource=tranquility&language=en", kind, typeId)
	resp, errGet := http.Get(url)

	if errGet != nil {
		log.Error("Unable to fetch for url %s \r\n", url)
	}

	b, errBody := ioutil.ReadAll(resp.Body)

	if errBody != nil {
		log.Errorf("Unable to fetch for url %s \r\n", url)
	}

	var item UniverseElementName
	json.Unmarshal(b, &item)

	if item.Name == "" {
		log.Errorf("Unable to fetch for url %s \r\n", url)
	}

	return item.Name
}

func (er *EsiRepository) GetItemsIdOnMarketForRegion(regionId int32) ([]int, error) {
	return []int{3977, 1957, 9371, 12217, 648}, nil

	// return getItemsOnMarketForRegion(int(regionId))
}

func getNbPages(url string) int {
	resp, err := http.Head(url)

	if err != nil {
		fmt.Println("Unable to fetch head: ", url)
	}

	nbPages, _ := strconv.ParseInt(resp.Header.Get("X-Pages"), 10, 32)

	return int(nbPages)
}

func getOrdersForRegionAndType(r, t int) ([]domain.RawOrder, error) {
	o := make([]domain.RawOrder, 0)
	headUrl := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/orders/?datasource=tranquility&order_type=all&page=1&type_id=%d", r, t)
	nbPages := getNbPages(headUrl)

	c := make(chan chanFetchOrders)

	for p := 1; p <= nbPages; p++ {
		go getOrdersForRegionAndTypeOnPage(r, t, p, c)
	}

	for i := 1; i <= nbPages; i++ {
		resp := <-c
		o = append(o, resp.orders...)
	}

	return o, nil
}

func getOrdersForRegionAndTypeOnPage(r int, t int, p int, c chan chanFetchOrders) {
	u := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/orders/?datasource=tranquility&order_type=all&page=%d&type_id=%d", r, p, t)

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

func getItemsOnMarketForRegion(r int) ([]int, error) {
	ids := make([]int, 0)
	headUrl := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/types/?datasource=tranquility&page=1", r)
	nbPages := getNbPages(headUrl)

	c := make(chan chanFetchItemsOnMarket)

	for p := 1; p <= nbPages; p++ {
		go getItemsOnMarketForRegionAndPage(r, p, c)
	}

	for i := 1; i <= nbPages; i++ {
		resp := <-c
		ids = append(ids, resp.ids...)
	}

	return ids, nil
}

func getItemsOnMarketForRegionAndPage(r int, p int, c chan chanFetchItemsOnMarket) {
	u := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/types/?datasource=tranquility&page=%d", r, p)

	resp, err := http.Get(u)

	if err != nil {
		fmt.Printf("Unable to fetch items for url %s", u)
		c <- chanFetchItemsOnMarket{err: true}
	}

	b, errBody := ioutil.ReadAll(resp.Body)

	if errBody != nil {
		fmt.Printf("Unable to fetch items for url %s", u)
		c <- chanFetchItemsOnMarket{err: true}
	}

	var ids []int
	json.Unmarshal(b, &ids)

	c <- chanFetchItemsOnMarket{ids: ids, url: u, err: false}
}
