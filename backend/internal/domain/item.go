package domain

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type ItemUseCase struct {
	itemRepository         ItemRepository
	externalItemRepository ExternalItemRepository
	notifier               ItemNotifier
}

func NewItemUseCase(itemRepository ItemRepository, externalItemRepository ExternalItemRepository, notifier ItemNotifier) ItemUseCase {
	return ItemUseCase{
		itemRepository:         itemRepository,
		externalItemRepository: externalItemRepository,
		notifier:               notifier,
	}
}

type ExternalItemRepository interface {
	GetItemsIdOnMarketForRegion(regionId int32) ([]int, error)
}

type ItemRepository interface {
	GetIntersectOnIndexWithItems(regionId int32, itemsIds []int) ([]int, []int, error)
	IsItemIndexForRegionId(regionId, itemId int32) (bool, error)
	AddItemInIndexForRegionId(regionId int32, itemId int32) error
	RemoveItemFromIndexForRegionId(regionId int32, itemId int32) error
}

type ItemNotifier interface {
	NotifyItemsIndexation(regionId int32, itemsIds []int, kind string) error
}

func (iuc *ItemUseCase) FindItemToFetch(regionId int32) error {
	log.Infoln("Get items on market")
	ids, errGet := iuc.externalItemRepository.GetItemsIdOnMarketForRegion(regionId)

	if errGet != nil {
		return fmt.Errorf("Cannot get items on market: %w", errGet)
	}
	log.Infof("%d items found", len(ids))

	log.Infoln("Find items not indexed")
	toIndex, toRemove, errFind := iuc.itemRepository.GetIntersectOnIndexWithItems(regionId, ids)

	if errFind != nil {
		return fmt.Errorf("Cannot get items intersection: %w", errFind)
	}

	log.Infof("%d items to index", len(toIndex))
	iuc.notifier.NotifyItemsIndexation(regionId, toIndex, "add")

	log.Infof("%d items to unindex", len(toRemove))
	iuc.notifier.NotifyItemsIndexation(regionId, toRemove, "remove")

	return nil
}

func (iuc *ItemUseCase) SetItemAsIndexedForRegionId(regionId, itemId int32) error {
	isInIndex, _ := iuc.itemRepository.IsItemIndexForRegionId(regionId, itemId)

	if !isInIndex {
		iuc.itemRepository.AddItemInIndexForRegionId(regionId, itemId)
	}

	return nil
}

func (iuc *ItemUseCase) RemoveItemFromIndexForRegionId(regionId, itemId int32) error {
	iuc.itemRepository.RemoveItemFromIndexForRegionId(regionId, itemId)

	return nil
}
