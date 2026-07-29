package gameservice

import (
	"testing"

	Item "gw1/server/item"
	"github.com/stretchr/testify/assert"
)

func TestBuildDBBags_Empty(t *testing.T) {
	im := NewItemMgr(nil)
	bags := im.BuildDBBags()
	assert.Empty(t, bags)
}

func TestBuildDBBags_WithBags(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	im.AddBag(9, 2)

	bags := im.BuildDBBags()
	assert.Len(t, bags, 2)
	assert.Equal(t, uint8(20), bags[0].Capacity)
	assert.Equal(t, uint8(1), bags[0].Type)
	assert.Equal(t, uint8(9), bags[1].Capacity)
	assert.Equal(t, uint8(2), bags[1].Type)
	assert.Len(t, bags[0].Slots, 20)
	assert.Len(t, bags[1].Slots, 9)
}

func TestBuildDBBags_WithItems(t *testing.T) {
	item := Item.GetItemDefinitionById(Item.ItemStarterHammer)
	im := NewItemMgr(nil)
	im.AddBag(20, 1)

	lid, err := im.AddItemToSlot(0, 0, item, Item.ItemStarterHammer, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, lid)

	bags := im.BuildDBBags()
	assert.Len(t, bags, 1)
	assert.Equal(t, uint32(Item.ItemStarterHammer), bags[0].Slots[0].ItemID)
	assert.Equal(t, uint32(1), bags[0].Slots[0].ItemQuantity)
}

func TestBuildDBBags_EmptySlots(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(5, 1)

	bags := im.BuildDBBags()
	assert.Len(t, bags, 1)
	assert.Len(t, bags[0].Slots, 5)
	// Empty slots should have ItemID=0
	for _, s := range bags[0].Slots {
		assert.Equal(t, uint32(0), s.ItemID)
	}
}

func TestBuildDBBags_MultipleBagsWithItems(t *testing.T) {
	hammer := Item.GetItemDefinitionById(Item.ItemStarterHammer)
	bow := Item.GetItemDefinitionById(Item.ItemStarterBow)
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddBag(5, 2)

	im.AddItemToSlot(0, 0, hammer, Item.ItemStarterHammer, 0)
	im.AddItemToSlot(1, 0, bow, Item.ItemStarterBow, 0)

	bags := im.BuildDBBags()
	assert.Len(t, bags, 2)

	assert.Equal(t, uint32(Item.ItemStarterHammer), bags[0].Slots[0].ItemID)
	assert.Equal(t, uint32(Item.ItemStarterBow), bags[1].Slots[0].ItemID)
}

func TestSyncToDB_DoesNotWriteWhenCharacterIdZero(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	err := im.SyncToDB()
	assert.NoError(t, err)
}
