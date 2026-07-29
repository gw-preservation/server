package gameservice

import (
	"fmt"
	"gw1/server/db"
	Item "gw1/server/item"
)

type slot struct {
	localId   int
	item      Item.Item
	itemId    Item.ItemId
	quantity  uint32
	dye       uint8
	modifiers []uint32
}

func emptySlot() slot {
	return slot{localId: -1}
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
		b.slots[index] = emptySlot()
		return true
	}
	return false
}

type ItemMgr struct {
	bags           []bag
	localIdCounter int
	player         *Player
	dbCharacterId  uint64
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

func (m *ItemMgr) Load(bags []db.Bag, characterId uint64) {
	m.bags = nil
	m.dbCharacterId = characterId
	for i, dbBag := range bags {
		b := bag{typ: int(dbBag.Type)}
		b.localId = m.getAndIncreaseLocalIdCounter()
		b.slots = make([]slot, len(dbBag.Slots))
		for i := range b.slots {
			b.slots[i] = emptySlot()
		}
		m.bags = append(m.bags, b)

		if m.player != nil {
			if int(dbBag.Type) == 1 {
				backpackDefinition := Item.GetItemDefinitionById(Item.ItemBackpack)
				m.player.EnqueuePacket(MarshalItemGeneralInfo(
					b.localId,
					int(backpackDefinition.ModelFileId()),
					int(backpackDefinition.Type()),
					1, 0, 0, 0,
					0x20001000,
					backpackDefinition.MerchValue(),
					32, 1,
					backpackDefinition.EncodeName(),
					backpackDefinition.MarshalModifiers(),
				))
				m.player.EnqueuePacket(MarshalInventoryCreateBag(1, int(dbBag.Type), 0, i, len(b.slots), b.localId))
				m.player.EnqueuePacket(MarshalItemUpdateName(b.localId, m.player.name))
			} else if int(dbBag.Type) == 2 {
				m.player.EnqueuePacket(MarshalInventoryCreateBag(1, int(dbBag.Type), 21, i, len(b.slots), 0))
			}
		}
	}
	for bagIdx, dbBag := range bags {
		for slotIdx, dbSlot := range dbBag.Slots {
			if dbSlot.ItemID == 0 {
				continue
			}
			itemDef := Item.GetItemDefinitionById(Item.ItemId(dbSlot.ItemID))
			lid := m.getAndIncreaseLocalIdCounter()
			m.bags[bagIdx].slots[slotIdx] = slot{
				localId:   lid,
				item:      itemDef,
				itemId:    Item.ItemId(dbSlot.ItemID),
				quantity:  dbSlot.ItemQuantity,
				dye:       dbSlot.Dye1,
				modifiers: []uint32(dbSlot.ItemModifiers),
			}
			if m.player != nil {
				m.player.EnqueuePacket(MarshalItemGeneralInfo(
					lid,
					int(itemDef.ModelFileId()),
					int(itemDef.Type()),
					1,
					int(dbSlot.Dye1),
					0, 0,
					itemDef.ComputeInteractionFlags(),
					itemDef.MerchValue(),
					lid,
					int(dbSlot.ItemQuantity),
					itemDef.EncodeName(),
					itemDef.MarshalModifiers(),
				))
				m.player.EnqueuePacket(MarshalItemMovedToLocation(1, lid, bagIdx, slotIdx))
			}
		}
	}
}

func (m *ItemMgr) BuildDBBags() []db.Bag {
	var dbBags []db.Bag
	for _, b := range m.bags {
		dbBag := db.Bag{
			CharacterID: m.dbCharacterId,
			Capacity:    uint8(len(b.slots)),
			Type:        uint8(b.typ),
		}
		dbSlots := make([]db.Slot, len(b.slots))
		for i, s := range b.slots {
			if s.localId == -1 {
				dbSlots[i] = db.Slot{BagID: dbBag.ID}
				continue
			}
			dbSlots[i] = db.Slot{
				BagID:         dbBag.ID,
				ItemID:        uint32(s.itemId),
				ItemQuantity:  s.quantity,
				ItemModifiers: db.ModifiersArray(s.modifiers),
				Dye1:          s.dye,
			}
			if dbSlots[i].ItemQuantity == 0 {
				dbSlots[i].ItemQuantity = 1
			}
		}
		dbBag.Slots = dbSlots
		dbBags = append(dbBags, dbBag)
	}
	return dbBags
}

func (m *ItemMgr) SyncToDB() error {
	if m.dbCharacterId == 0 {
		return nil
	}
	return db.ReplaceBagsForCharacter(m.dbCharacterId, m.BuildDBBags())
}

func (m *ItemMgr) AddBag(capacity int, typ int) {
	b := bag{typ: typ}
	b.localId = m.getAndIncreaseLocalIdCounter()
	b.slots = make([]slot, capacity)
	for i := range b.slots {
		b.slots[i] = emptySlot()
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
		if m.player != nil {
			m.player.EnqueuePacket(MarshalRemoveItem(1, origLid))
		}
	}
	return nil
}

func (m *ItemMgr) MoveItemByLocalId(localId int, containerIndex, slotIndex int) error {
	if containerIndex >= len(m.bags) {
		return fmt.Errorf("MoveItemByLocalId: containerIndex(%d) >= len(m.bags)(%d)", containerIndex, len(m.bags))
	}
	if slotIndex >= len(m.bags[containerIndex].slots) {
		return fmt.Errorf("MoveItemByLocalId: slotIndex(%d) >= len(slots)(%d)", slotIndex, len(m.bags[containerIndex].slots))
	}
	origBagIndex, ok := m.GetBagIndexForLocalId(localId)
	if !ok {
		return fmt.Errorf("Unable to find bag index for item local id")
	}
	origSlotIndex, ok := m.GetSlotIndexForLocalId(localId)
	if !ok {
		return fmt.Errorf("Unable to find slot index for item local id")
	}
	m.bags[containerIndex].slots[slotIndex] = m.bags[origBagIndex].slots[origSlotIndex]
	m.bags[origBagIndex].slots[origSlotIndex] = emptySlot()
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
	lid := m.getAndIncreaseLocalIdCounter()
	m.bags[containerIndex].slots[slotIndex] = slot{
		localId:  lid,
		item:     item,
		itemId:   itemId,
		quantity: 1,
		dye:      uint8(dye),
	}
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
		m.player.EnqueuePacket(MarshalItemMovedToLocation(1, lid, containerIndex, slotIndex))
	}
	return lid, nil
}

func (m *ItemMgr) GetNumBags() int {
	return len(m.bags)
}

func (m *ItemMgr) UpdateSlotDye(containerIndex, slotIndex int, dye uint8) {
	if containerIndex >= len(m.bags) {
		return
	}
	if slotIndex >= len(m.bags[containerIndex].slots) {
		return
	}
	m.bags[containerIndex].slots[slotIndex].dye = dye
}
