package GameService

import (
	Item "gw1/server/item"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testItem = Item.GetItemDefinitionById(Item.ItemStarterHammer)

func TestNewItemMgr_CounterStartsAt1(t *testing.T) {
	im := NewItemMgr(nil)
	assert.Equal(t, 1, im.localIdCounter)
}

func TestAddBag_IncrementsCounter(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	assert.Equal(t, 2, im.localIdCounter)
	im.AddBag(5, 1)
	assert.Equal(t, 3, im.localIdCounter)
}

func TestAddBag_SetsTypeAndLocalId(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	assert.Equal(t, 1, im.bags[0].localId)
	assert.Equal(t, 1, im.bags[0].typ)

	im.AddBag(5, 2)
	assert.Equal(t, 2, im.bags[1].localId)
	assert.Equal(t, 2, im.bags[1].typ)
}

func TestGetNumBags(t *testing.T) {
	im := NewItemMgr(nil)
	assert.Equal(t, 0, im.GetNumBags())
	im.AddBag(10, 1)
	assert.Equal(t, 1, im.GetNumBags())
	im.AddBag(5, 2)
	assert.Equal(t, 2, im.GetNumBags())
}

func TestGetNumSlotsInBag(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(20, 1)
	im.AddBag(5, 1)
	assert.Equal(t, 20, im.GetNumSlotsInBag(0))
	assert.Equal(t, 5, im.GetNumSlotsInBag(1))
}

func TestGetBagType(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddBag(10, 2)
	assert.Equal(t, 1, im.GetBagType(0))
	assert.Equal(t, 2, im.GetBagType(1))
}

func TestGetBagLocalId(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddBag(10, 1)
	assert.Equal(t, 1, im.GetBagLocalId(0))
	assert.Equal(t, 2, im.GetBagLocalId(1))
}

func TestHasItemInSlot_Empty(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	assert.False(t, im.HasItemInSlot(0, 0))
}

func TestHasItemInSlot_Occupied(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	assert.True(t, im.HasItemInSlot(0, 0))
}

func TestHasItemInSlot_OutOfBounds(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	assert.False(t, im.HasItemInSlot(5, 0))
	assert.False(t, im.HasItemInSlot(0, 50))
}

func TestGetLocalIdForSlot_Empty(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	id, err := im.GetLocalIdForSlot(0, 0)
	assert.NoError(t, err)
	assert.Equal(t, -1, id)
}

func TestGetLocalIdForSlot_Occupied(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	id, err := im.GetLocalIdForSlot(0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, id)
}

func TestGetLocalIdForSlot_OutOfBounds(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	_, err := im.GetLocalIdForSlot(5, 0)
	assert.Error(t, err)
	_, err = im.GetLocalIdForSlot(0, 50)
	assert.Error(t, err)
}

func TestGetItemInSlot_Empty(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	item, err := im.GetItemInSlot(0, 0)
	assert.NoError(t, err)
	assert.Equal(t, Item.Item{}, item)
}

func TestGetItemInSlot_Occupied(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	item, err := im.GetItemInSlot(0, 0)
	assert.NoError(t, err)
	assert.Equal(t, testItem, item)
}

func TestGetItemInSlot_OutOfBounds(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	_, err := im.GetItemInSlot(5, 0)
	assert.Error(t, err)
	_, err = im.GetItemInSlot(0, 50)
	assert.Error(t, err)
}

func TestAddItemToSlot_Success(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	lid, err := im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, lid)
	assert.True(t, im.HasItemInSlot(0, 0))

	item, _ := im.GetItemInSlot(0, 0)
	assert.Equal(t, testItem, item)
}

func TestAddItemToSlot_SlotOccupied(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	_, err := im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	assert.Error(t, err)
}

func TestAddItemToSlot_OutOfBounds(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	_, err := im.AddItemToSlot(5, 0, testItem, Item.ItemStarterHammer, 0)
	assert.Error(t, err)
	_, err = im.AddItemToSlot(0, 50, testItem, Item.ItemStarterHammer, 0)
	assert.Error(t, err)
}

func TestRemoveItemInSlot_HasItem(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	im.bags[0].RemoveItemInSlot(0)
	assert.False(t, im.HasItemInSlot(0, 0))

	id, _ := im.GetLocalIdForSlot(0, 0)
	assert.Equal(t, -1, id)
}

func TestRemoveItemInSlot_Empty(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.bags[0].RemoveItemInSlot(0)
	assert.False(t, im.HasItemInSlot(0, 0))
}

func TestRemoveItemInSlot_OutOfBounds(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.bags[0].RemoveItemInSlot(50)
}

func TestRemoveItemByLocalId_Success(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	lid, _ := im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	err := im.RemoveItemByLocalId(lid)
	assert.NoError(t, err)
	assert.False(t, im.HasItemInSlot(0, 0))
}

func TestRemoveItemByLocalId_NotFound(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	err := im.RemoveItemByLocalId(9999)
	assert.Error(t, err)
}

func TestMoveItemByLocalId_Success(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddBag(10, 1)
	lid, _ := im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)

	err := im.MoveItemByLocalId(lid, 1, 3)
	assert.NoError(t, err)
	assert.False(t, im.HasItemInSlot(0, 0))
	assert.True(t, im.HasItemInSlot(1, 3))

	item, _ := im.GetItemInSlot(1, 3)
	assert.Equal(t, testItem, item)
}

func TestMoveItemByLocalId_NotFound(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	err := im.MoveItemByLocalId(9999, 0, 0)
	assert.Error(t, err)
}

func TestLocalIds_UniqueAcrossBags(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)
	im.AddBag(10, 1)

	lid1, _ := im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	lid2, _ := im.AddItemToSlot(1, 0, testItem, Item.ItemStarterHammer, 0)
	lid3, _ := im.AddItemToSlot(0, 1, testItem, Item.ItemStarterHammer, 0)

	assert.NotEqual(t, lid1, lid2)
	assert.NotEqual(t, lid1, lid3)
	assert.NotEqual(t, lid2, lid3)
}

func TestLocalIds_NoReuseAfterRemove(t *testing.T) {
	im := NewItemMgr(nil)
	im.AddBag(10, 1)

	lid1, _ := im.AddItemToSlot(0, 0, testItem, Item.ItemStarterHammer, 0)
	im.RemoveItemByLocalId(lid1)
	lid2, _ := im.AddItemToSlot(0, 1, testItem, Item.ItemStarterHammer, 0)

	assert.NotEqual(t, lid1, lid2)
	assert.Greater(t, lid2, lid1)
}
