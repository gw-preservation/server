package Item

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetItemDefinitionById_Exists(t *testing.T) {
	item := GetItemDefinitionById(ItemStarterHammer)
	assert.Equal(t, "StarterHammer", item.Name())
}

func TestGetItemDefinitionById_PanicsOnMissing(t *testing.T) {
	assert.Panics(t, func() {
		GetItemDefinitionById(99999)
	})
}

func TestName(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Equal(t, "Backpack", item.Name())
}

func TestType(t *testing.T) {
	item := GetItemDefinitionById(ItemStarterHammer)
	assert.Equal(t, ItemTypeHammer, item.Type())
}

func TestMerchValue(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Equal(t, 5, item.MerchValue())
}

func TestModelFileId(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Equal(t, 0x1b536, item.ModelFileId())
}

func TestRarity(t *testing.T) {
	item := GetItemDefinitionById(ItemSummoningStone)
	assert.Equal(t, RarityPurple, item.Rarity())

	item = GetItemDefinitionById(ItemEverlastingGhostlyStaff)
	assert.Equal(t, RarityGreen, item.Rarity())

	item = GetItemDefinitionById(ItemEternalBlade)
	assert.Equal(t, RarityGold, item.Rarity())
}

func TestEncName(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Equal(t, "21a8 d157 b58f 166f", item.EncName())

	item = GetItemDefinitionById(ItemEternalBlade)
	assert.Equal(t, "", item.EncName())
}

func TestInherentMods(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Len(t, item.InherentMods(), 1)

	item = GetItemDefinitionById(ItemStarterHammer)
	assert.Len(t, item.InherentMods(), 0)
}

func TestMarshalModifiers(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	mods := item.MarshalModifiers()
	assert.Len(t, mods, 1)
	assert.IsType(t, uint32(0), mods[0])
}

func TestGetEquipSlot_Weapons(t *testing.T) {
	tests := []struct {
		id       ItemId
		expected EquipSlot
	}{
		{ItemStarterHammer, EquipSlotRightHand},
		{ItemStarterBow, EquipSlotRightHand},
		{ItemStarterCane, EquipSlotRightHand},
		{ItemEternalBlade, EquipSlotRightHand},
		{ItemEternalShield, EquipSlotLeftHand},
	}
	for _, tt := range tests {
		item := GetItemDefinitionById(tt.id)
		assert.Equal(t, tt.expected, item.GetEquipSlot(), "item %d", tt.id)
	}
}

func TestGetEquipSlot_Armor(t *testing.T) {
	tests := []struct {
		id       ItemId
		expected EquipSlot
	}{
		{ItemRingmailHauberk, EquipSlotChest},
		{ItemRingmailLeggings, EquipSlotLegs},
		{ItemRingmailBoots, EquipSlotBoots},
		{ItemRingmailGauntlets, EquipSlotGloves},
		{ItemRecruitsCap, EquipSlotHead},
	}
	for _, tt := range tests {
		item := GetItemDefinitionById(tt.id)
		assert.Equal(t, tt.expected, item.GetEquipSlot(), "item %d", tt.id)
	}
}

func TestGetEquipSlot_Unknown(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Equal(t, EquipSlotUnknown, item.GetEquipSlot())
}

func TestGetVisualEquipSlot_Weapons(t *testing.T) {
	tests := []struct {
		id       ItemId
		expected EquipVisualSlot
	}{
		{ItemStarterHammer, EquipVisualSlotRightHand},
		{ItemEternalBlade, EquipVisualSlotRightHand},
		{ItemEternalShield, EquipVisualSlotLeftHand},
	}
	for _, tt := range tests {
		item := GetItemDefinitionById(tt.id)
		assert.Equal(t, tt.expected, item.GetVisualEquipSlot(), "item %d", tt.id)
	}
}

func TestGetVisualEquipSlot_Armor(t *testing.T) {
	tests := []struct {
		id       ItemId
		expected EquipVisualSlot
	}{
		{ItemRingmailHauberk, EquipVisualSlotChest},
		{ItemRingmailLeggings, EquipVisualSlotLegs},
		{ItemRingmailBoots, EquipVisualSlotBoots},
		{ItemRingmailGauntlets, EquipVisualSlotGloves},
		{ItemRecruitsCap, EquipVisualSlotHead},
	}
	for _, tt := range tests {
		item := GetItemDefinitionById(tt.id)
		assert.Equal(t, tt.expected, item.GetVisualEquipSlot(), "item %d", tt.id)
	}
}

func TestGetVisualEquipSlot_Unknown(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	assert.Equal(t, EquipVisualSlotUnknown, item.GetVisualEquipSlot())
}

func TestGetRarityFlag(t *testing.T) {
	assert.Equal(t, 0, GetRarityFlag(RarityWhite))
	assert.Equal(t, 0, GetRarityFlag(RarityBlue))
	assert.Equal(t, 0x400000, GetRarityFlag(RarityPurple))
	assert.Equal(t, 0x20000, GetRarityFlag(RarityGold))
	assert.Equal(t, 0x10, GetRarityFlag(RarityGreen))
}

func TestComputeInteractionFlags_Weapon(t *testing.T) {
	item := GetItemDefinitionById(ItemStarterHammer)
	flags := item.ComputeInteractionFlags()
	assert.Equal(t, 0x20000000, flags)
}

func TestComputeInteractionFlags_Armor(t *testing.T) {
	item := GetItemDefinitionById(ItemRingmailHauberk)
	flags := item.ComputeInteractionFlags()
	assert.Equal(t, 0x20000004, flags) // base | (1 << 2) = armor flag
}

func TestComputeInteractionFlags_ThirdEye(t *testing.T) {
	item := GetItemDefinitionById(ItemThirdEye)
	flags := item.ComputeInteractionFlags()
	assert.Equal(t, 0x20001202, flags)
}

func TestComputeInteractionFlags_StarterTruncheon(t *testing.T) {
	item := GetItemDefinitionById(ItemStarterTruncheon)
	flags := item.ComputeInteractionFlags()
	assert.Equal(t, 0x22001000, flags)
}

func TestEncodeName_Encoded(t *testing.T) {
	item := GetItemDefinitionById(ItemBackpack)
	enc := item.EncodeName()
	assert.NotEmpty(t, enc)
	// Encoded name should have bytes for each hex word + prefix
	// "21a8 d157 b58f 166f" = 4 words = 8 bytes
	assert.Equal(t, 8, len(enc))
}

func TestEncodeName_Fallback(t *testing.T) {
	item := Item{name: "Short", encName: ""}
	enc := item.EncodeName()
	assert.NotEmpty(t, enc)
	// Fallback: prefix (0x0108, 0x0107) + UTF16 "Short" (5) + terminator (0x0001) = 8 units = 16 bytes
	assert.Equal(t, 16, len(enc))
}

func TestEncodeName_FallbackLong(t *testing.T) {
	item := Item{name: "StarterHammer", encName: ""}
	enc := item.EncodeName()
	assert.NotEmpty(t, enc)
	// Fallback: 2 prefix + 13 chars + 1 terminator = 16 units = 32 bytes
	assert.Equal(t, 32, len(enc))
}

func TestDefaultEquipmentSlices(t *testing.T) {
	assert.Len(t, DefaultEquipmentWarrior, 6)
	assert.Len(t, DefaultEquipmentRanger, 6)
	assert.Len(t, DefaultEquipmentMonk, 6)
	assert.Len(t, DefaultEquipmentNecromancer, 6)
	assert.Len(t, DefaultEquipmentMesmer, 6)
	assert.Len(t, DefaultEquipmentElementalist, 6)
}
