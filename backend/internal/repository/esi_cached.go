package repository

import "fmt"

type CacheClient interface {
	Save(key, value string) error
	Get(key string) (string, bool)
}

type EsiRepositoryWithCache struct {
	Cache CacheClient
	Esi   EsiRepository
}

func (er *EsiRepositoryWithCache) FetchTypeName(typeId int32) string {
	return er.fetchElementNameById(typeId, "types")
}

func (er *EsiRepositoryWithCache) FetchRegionName(regionId int32) string {
	return er.fetchElementNameById(regionId, "regions")
}

func (er *EsiRepositoryWithCache) FetchSystemName(systemId int32) string {
	return er.fetchElementNameById(systemId, "systems")
}

func (er *EsiRepositoryWithCache) FetchLocationName(locationId int32) string {
	return er.fetchElementNameById(locationId, "stations")
}

func (er *EsiRepositoryWithCache) fetchElementNameById(typeId int32, kind string) string {
	key := fmt.Sprintf("%s:%d", kind, typeId)
	val, exist := er.Cache.Get(fmt.Sprint(key))

	if exist {
		return val
	}

	name := er.Esi.FetchElementName(typeId, kind)

	er.Cache.Save(key, name)

	return name
}
