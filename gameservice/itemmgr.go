package GameService

import (
	"fmt"
	Item "gw1/server/item"
)

type slot struct {
	localId int
	item    Item.Item
}

type bag struct {
	localId int
	typ     int
	slots   []slot
}

func (b *bag) GetNumSlots() int {
	return len(b.slots)
}

func (b *bag) RemoveItemInSlot(index int) bool {
	if index >= len(b.slots) {
		return false
	}
	if b.slots[index].localId != -1 {
		b.slots[index].localId = -1
		return true
	}
	return false
}

type ItemMgr struct {
	bags           []bag
	localIdCounter int
	player         *Player
}

func NewItemMgr(p *Player) ItemMgr {
	mgr := ItemMgr{
		localIdCounter: 1,
		player:         p,
	}
	return mgr
}

func (m *ItemMgr) getAndIncreaseLocalIdCounter() int {
	counter := m.localIdCounter
	m.localIdCounter++
	return counter
}

func (m *ItemMgr) AddBag(capacity int, typ int) {
	b := bag{typ: typ}
	b.localId = m.getAndIncreaseLocalIdCounter()
	b.slots = make([]slot, capacity)
	for i := range b.slots {
		b.slots[i].localId = -1
	}
	if m.player != nil {
		if typ == 1 {
			// Inventory
			backpackDefinition := Item.GetItemDefinitionById(Item.ItemBackpack)
			m.player.EnqueuePacket(MarshalItemGeneralInfo(
				b.localId,
				int(backpackDefinition.ModelFileId()),
				int(backpackDefinition.Type()),
				1,
				0,
				0,
				0,
				0x20001000,
				backpackDefinition.MerchValue(),
				32,
				1,
				backpackDefinition.EncodeName(),
				backpackDefinition.MarshalModifiers(),
			))
			m.player.EnqueuePacket(MarshalInventoryCreateBag(1, typ, 0, len(m.bags), capacity, b.localId))
			m.player.EnqueuePacket(MarshalItemUpdateName(b.localId, m.player.name))
		} else if typ == 2 {
			// Equipped
			m.player.EnqueuePacket(MarshalInventoryCreateBag(1, typ, 21, len(m.bags), capacity, 0))
		}
	}
	m.bags = append(m.bags, b)
}

func (m *ItemMgr) GetNumSlotsInBag(containerIndex int) int {
	return len(m.bags[containerIndex].slots)
}

func (m *ItemMgr) GetBagType(containerIndex int) int {
	return m.bags[containerIndex].typ
}
func (m *ItemMgr) GetBagLocalId(containerIndex int) int {
	return m.bags[containerIndex].localId
}

func (m *ItemMgr) HasItemInSlot(containerIndex int, slotIndex int) bool {
	if containerIndex >= len(m.bags) {
		return false
	}
	container := m.bags[containerIndex]
	if slotIndex >= len(container.slots) {
		return false
	}
	return container.slots[slotIndex].localId > -1
}

func (m *ItemMgr) GetLocalIdForSlot(containerIndex, slotIndex int) (int, error) {
	if containerIndex >= len(m.bags) {
		return 0, fmt.Errorf("GetLocalIdForSlot: containerIndex(%d) >= len(m.bags)(%d)", containerIndex, len(m.bags))
	}
	container := m.bags[containerIndex]
	if slotIndex >= len(container.slots) {
		return 0, fmt.Errorf("GetLocalIdForSlot: slotIndex(%d) >= len(container.slots)(%d)", slotIndex, len(container.slots))
	}
	return container.slots[slotIndex].localId, nil
}

func (m *ItemMgr) GetItemInSlot(containerIndex int, slotIndex int) (Item.Item, error) {
	if containerIndex >= len(m.bags) {
		return Item.Item{}, fmt.Errorf("GetItemInSlot: containerIndex(%d) >= len(m.bags)(%d)", containerIndex, len(m.bags))
	}
	container := m.bags[containerIndex]
	if slotIndex >= len(container.slots) {
		return Item.Item{}, fmt.Errorf("GetItemInSlot: slotIndex(%d) >= len(container.slots)(%d)", slotIndex, len(container.slots))
	}
	return container.slots[slotIndex].item, nil
}

func (m *ItemMgr) GetItemByLocalId(localId int) (Item.Item, bool) {
	for _, bag := range m.bags {
		for _, slot := range bag.slots {
			if slot.localId == localId {
				return slot.item, true
			}
		}
	}
	return Item.Item{}, false
}

func (m *ItemMgr) GetBagIndexForLocalId(localId int) (int, bool) {
	for idx, bag := range m.bags {
		for _, slot := range bag.slots {
			if slot.localId == localId {
				return idx, true
			}
		}
	}
	return 0, false
}

func (m *ItemMgr) GetSlotIndexForLocalId(localId int) (int, bool) {
	for _, bag := range m.bags {
		for slotIndex, slot := range bag.slots {
			if slot.localId == localId {
				return slotIndex, true
			}
		}
	}
	return 0, false
}

func (m *ItemMgr) RemoveItemByLocalId(localId int) error {
	origBagIndex, ok := m.GetBagIndexForLocalId(localId)
	if !ok {
		return fmt.Errorf("Unable to find bag index for item local id")
	}
	origSlotIndex, ok := m.GetSlotIndexForLocalId(localId)
	if !ok {
		return fmt.Errorf("Unable to find slot index for item local id")
	}
	return m.RemoveItemInSlot(origBagIndex, origSlotIndex)
}

func (m *ItemMgr) RemoveItemInSlot(containerIndex, slotIndex int) error {
	if containerIndex >= len(m.bags) {
		return fmt.Errorf("RemoveItemInSlot: containerIndex(%d) >= len(m.bags)(%d)", containerIndex, len(m.bags))
	}
	container := m.bags[containerIndex]
	if slotIndex >= len(container.slots) {
		return fmt.Errorf("RemoveItemInSlot: slotIndex(%d) >= len(container.slots)(%d)", slotIndex, len(container.slots))
	}
	origLid := m.bags[containerIndex].slots[slotIndex].localId
	if container.RemoveItemInSlot(slotIndex) {
		m.player.EnqueuePacket(MarshalRemoveItem(1, origLid))
	}
	return nil
}

func (m *ItemMgr) MoveItemByLocalId(localId int, containerIndex, slotIndex int) error {
	origBagIndex, ok := m.GetBagIndexForLocalId(localId)
	if !ok {
		return fmt.Errorf("Unable to find bag index for item local id")
	}
	origSlotIndex, ok := m.GetSlotIndexForLocalId(localId)
	if !ok {
		return fmt.Errorf("Unable to find slot index for item local id")
	}
	m.bags[containerIndex].slots[slotIndex] = m.bags[origBagIndex].slots[origSlotIndex]
	m.bags[origBagIndex].slots[origSlotIndex].localId = -1
	if m.player != nil {
		m.player.EnqueuePacket(MarshalItemChangeLocation(1, localId, containerIndex, slotIndex))
	}
	return nil
}

func (m *ItemMgr) AddItemToSlot(containerIndex int, slotIndex int, item Item.Item, itemId Item.ItemId, dye int) (int, error) {
	if containerIndex >= len(m.bags) {
		return 0, fmt.Errorf("AddItemToSlot: containerIndex(%d) >= len(m.bags)(%d)", containerIndex, len(m.bags))
	}
	container := m.bags[containerIndex]
	if slotIndex >= len(container.slots) {
		return 0, fmt.Errorf("AddItemToSlot: slotIndex(%d) >= len(container.slots)(%d)", slotIndex, len(container.slots))
	}
	if m.HasItemInSlot(containerIndex, slotIndex) {
		return 0, fmt.Errorf("AddItemToSlot: there is already something there!")
	}
	m.bags[containerIndex].slots[slotIndex] = slot{
		m.getAndIncreaseLocalIdCounter(),
		item,
	}
	lid := m.bags[containerIndex].slots[slotIndex].localId
	if m.player != nil {
		m.player.EnqueuePacket(MarshalItemGeneralInfo(
			lid,
			int(item.ModelFileId()),
			int(item.Type()),
			1,
			dye,
			0,
			0,
			item.ComputeInteractionFlags(),
			item.MerchValue(),
			lid,
			1,
			item.EncodeName(),
			item.MarshalModifiers(),
		))
		m.player.EnqueuePacket(MarshalItemMovedToLocation(1, m.bags[containerIndex].slots[slotIndex].localId, containerIndex, slotIndex))
	}
	return lid, nil
}

func (m *ItemMgr) GetNumBags() int {
	return len(m.bags)
}
