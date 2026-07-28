package GameService

import (
	Item "gw1/server/item"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddBagIncreasesLocalIdCounter(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(100, 1)
	assert.Equal(t, 2, im.localIdCounter)
	im.AddBag(5, 1)
	assert.Equal(t, 3, im.localIdCounter)
}

func TestAddBagPreAllocatesSlots(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	assert.Len(t, im.bags[0].slots, 20)
	im.AddBag(5, 1)
	assert.Len(t, im.bags[1].slots, 5)
	// they have an empty localId by default
	for _, sl := range im.bags[0].slots {
		assert.Equal(t, -1, sl.localId)
	}
}

func TestHasItemInSlot(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	im.AddItemToSlot(0, 0, Item.GetItemDefinitionById(Item.ItemStarterHammer), Item.ItemStarterHammer, 0)
	assert.True(t, im.HasItemInSlot(0, 0))
}

func TestHasItemInSlotBadInput(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	im.AddItemToSlot(0, 0, Item.GetItemDefinitionById(Item.ItemStarterHammer), Item.ItemStarterHammer, 0)
	// bag index too large
	assert.False(t, im.HasItemInSlot(1, 0))
	// slot index too large
	assert.False(t, im.HasItemInSlot(0, 50))
}

func TestGetLocalIdForSlot(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	im.AddItemToSlot(0, 0, Item.GetItemDefinitionById(Item.ItemStarterHammer), Item.ItemStarterHammer, 0)
	id, err := im.GetLocalIdForSlot(0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, id)
}

func TestGetLocalIdForSlotBadInput(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	im.AddItemToSlot(0, 0, Item.GetItemDefinitionById(Item.ItemStarterHammer), Item.ItemStarterHammer, 0)
	_, err := im.GetLocalIdForSlot(1, 0)
	assert.Error(t, err)
	_, err = im.GetLocalIdForSlot(0, 21)
	assert.Error(t, err)
}
