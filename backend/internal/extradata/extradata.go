package extradata

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	goredis "github.com/go-redis/redis/v8"
	"github.com/panjf2000/ants/v2"
)

type UniverseElementName struct {
	Name string `json:"name"`
}

func FetchExtraData(extraData map[string]map[int]string, client *goredis.Client) map[string]map[int]string {
	pool, _ := ants.NewPoolWithFunc(50, taskFetchExtraDataHandler)
	defer pool.Release()

	var wg sync.WaitGroup

	tasks := make([]*taskFetchDataPayload, 0)
	for kind := range extraData {
		for id := range extraData[kind] {
			wg.Add(1)
			var value string
			task := &taskFetchDataPayload{
				wg:     &wg,
				kind:   kind,
				id:     id,
				client: client,
				value:  &value,
			}
			tasks = append(tasks, task)
			pool.Invoke(task)
		}
	}

	wg.Wait()

	for _, task := range tasks {
		if task.value == nil {
			continue
		}

		extraData[task.kind][task.id] = *task.value
	}

	return extraData
}

func getElementName(typeId int, kind string) (string, error) {
	url := fmt.Sprintf("https://esi.evetech.net/latest/universe/%s/%d/?datasource=tranquility&language=en", kind, typeId)
	resp, errGet := http.Get(url)

	if errGet != nil {
		fmt.Println(errGet, url)
		return "", fmt.Errorf("Unable to fetch for url %s: %w", url, errGet)
	}

	b, errBody := ioutil.ReadAll(resp.Body)

	if errBody != nil {
		fmt.Println(errGet, url)
		return "", fmt.Errorf("Unable to fetch for url %s: %w", url, errBody)
	}

	var item UniverseElementName
	json.Unmarshal(b, &item)

	if item.Name == "" {
		fmt.Println("error: ", url)
		return "", fmt.Errorf("No name for url %s", url)
	}

	return item.Name, nil
}

func taskFetchExtraDataHandler(data interface{}) {
	t := data.(*taskFetchDataPayload)
	t.fetch()
}

type taskFetchDataPayload struct {
	wg     *sync.WaitGroup
	kind   string
	id     int
	value  *string
	client *goredis.Client
}

func (t *taskFetchDataPayload) fetch() {
	val, errGet := t.client.Get(context.Background(), fmt.Sprintf("%s:%d", t.kind, t.id)).Result()

	if errGet == nil || val == "" {
		val, _ = getElementName(t.id, t.kind)

		t.client.Set(context.Background(), fmt.Sprintf("%s:%d", t.kind, t.id), val, 0)
	}

	t.value = &val
	t.wg.Done()
}
