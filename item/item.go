package Item

import (
	"fmt"
)

func (i Item) MarshalModifiers() (out []uint32) {
	for _, mod := range i.inherentMods {
		out = append(out, uint32(mod))
	}
	return
}

type Item struct {
	name         string
	typ          ItemType
	merchValue   int
	modelFileId  int
	rarity       Rarity
	encName      string
	inherentMods []int
}

func (i *Item) Name() string {
	return i.name
}

func (i *Item) Type() ItemType {
	return i.typ
}
func (i *Item) MerchValue() int {
	return i.merchValue
}
func (i *Item) ModelFileId() int {
	return i.modelFileId
}
func (i *Item) Rarity() Rarity {
	return i.rarity
}
func (i *Item) EncName() string {
	return i.encName
}
func (i *Item) InherentMods() []int {
	return i.inherentMods
}

type ItemType int

const (
	ItemTypeBag ItemType = iota
	ItemTypeConsumable
	ItemTypeCostume
	ItemTypeStaff
	ItemTypeLongbow
)

type Rarity int

const (
	RarityWhite Rarity = iota
	RarityBlue
	RarityPurple
	RarityGold
	RarityGreen
)

const (
	DamageTypeUnknown = iota
	DamageTypePiercing
	DamageTypeSlashing
	DamageTypeCold
	DamageTypeLightning
	DamageTypeFire
	DamageTypeChaos
	DamageTypeDark
	DamageTypeHoly
	DamageTypeNature
	DamageTypeSacrifice
	DamageTypeEarth
	DamageTypeGeneric
	DamageTypeDark2
)

type ItemId int

const (
	ItemBackpack                ItemId = 32
	ItemStarterBow              ItemId = 447
	ItemSummoningStone          ItemId = 30847
	ItemEverlastingGhostlyStaff ItemId = 729
)

var itemDefinitions map[ItemId]Item = map[ItemId]Item{
	ItemBackpack: {"Backpack", ItemTypeBag, 5, 0x1b536, RarityWhite, "21a8 d157 b58f 166f", []int{
		HoldsItems(20),
	}},
	ItemStarterBow: {"Starter Bow", ItemTypeLongbow, 1000, 0x24a4, RarityWhite, "223f 87c0 c25c 378e", []int{
		TwoHandedBow(3),
	}},
	ItemSummoningStone: {"Summoning Stone", ItemTypeConsumable, 0, 0x54992, RarityPurple, "8102 4674 F109 F330 2937", []int{
		SummonStoneModifier(136),
	}},
	ItemEverlastingGhostlyStaff: {"Everlasting Ghostly Staff", ItemTypeStaff, 0, 0x2141d, RarityGreen, "8101 3C07 B4E7 E010 6B5E", []int{
		AttributeRequirement(12, 9),
		DamageType(DamageTypeChaos),
		HalfSkillRecharge(20),
		EnchantDuration(20),
		ExtraEnergyHealthOver(5, 50),
		ExtraEnergy(15),
		DamageRange(11, 22),
	}},
}

func GetItemDefinitionById(id ItemId) (item Item) {
	var ok bool
	item, ok = itemDefinitions[id]
	if !ok {
		panic(fmt.Sprintf("GetItemDefinitionById(%d): no definition!", id))
	}
	return
}

func (i *Item) ComputeInteractionFlags() int {
	flags := 0x20000000
	flags |= GetRarityFlag(i.rarity)
	return flags
}

func GetRarityFlag(rarity Rarity) int {
	switch rarity {
	case RarityPurple:
		return 0x400000
	case RarityGreen:
		return 0x10
	case RarityGold:
		return 0x20000
	}
	return 0
}
