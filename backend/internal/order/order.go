package order

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"github.com/panjf2000/ants/v2"
)

type Order struct {
	IsBuyOrder  bool    `json:"is_buy_order"`
	LocationId  int     `json:"location_id"`
	Price       float64 `json:"price"`
	SystemId    int     `json:"system_id"`
	TypeId      int     `json:"type_id"`
	VolumeTotal int     `json:"volume_total"`
	IssuedAt    string  `json:"issued"`
	OrderId     int     `json:"order_id"`
}

func GetOrdersFromEsiForRegion(regionId int) []Order {
	headUrl := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/orders/?datasource=tranquility&order_type=all&page=1", regionId)
	nbPages := getNbPages(headUrl)

	pool, _ := ants.NewPoolWithFunc(20, taskGetOrderForPageHandler)
	defer pool.Release()

	var wg sync.WaitGroup
	tasks := make([]*taskGetOrderForPagePayload, 0)

	for p := 1; p <= nbPages; p++ {
		wg.Add(1)
		task := &taskGetOrderForPagePayload{
			wg:       &wg,
			page:     p,
			regionId: regionId,
		}

		tasks = append(tasks, task)
		pool.Invoke(task)
	}

	wg.Wait()

	orders := make([]Order, 0)
	for _, task := range tasks {
		if task.err {
			continue
		}

		orders = append(orders, task.orders...)
	}

	return orders
}

func taskGetOrderForPageHandler(data interface{}) {
	t := data.(*taskGetOrderForPagePayload)
	t.fetchPage()
}

type taskGetOrderForPagePayload struct {
	wg       *sync.WaitGroup
	page     int
	orders   []Order
	regionId int
	err      bool
}

func (t *taskGetOrderForPagePayload) fetchPage() {
	u := fmt.Sprintf("https://esi.evetech.net/latest/markets/%d/orders/?datasource=tranquility&order_type=all&page=%d", t.regionId, t.page)

	resp, errGet := http.Get(u)

	if errGet != nil {
		t.err = true
		t.wg.Done()
	}

	b, errBody := ioutil.ReadAll(resp.Body)

	if errBody != nil {
		t.err = true
		t.wg.Done()
	}

	var orders []Order
	json.Unmarshal(b, &orders)
	t.orders = orders

	t.wg.Done()
}

func getNbPages(url string) int {
	resp, err := http.Head(url)

	if err != nil {
		return 0
	}

	nbPages, _ := strconv.ParseInt(resp.Header.Get("X-Pages"), 10, 32)

	return int(nbPages)
}
