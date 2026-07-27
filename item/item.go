package Item

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
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

func (i *Item) GetEquipSlot() EquipSlot {
	switch i.typ {
	case ItemTypeWand, ItemTypeStaff, ItemTypeSword, ItemTypeHammer, ItemTypeLongbow:
		return EquipSlotRightHand
	case ItemTypeShield:
		return EquipSlotLeftHand
	case ItemTypeArmorGloves:
		return EquipSlotGloves
	case ItemTypeArmorBoots:
		return EquipSlotBoots
	case ItemTypeArmorLegs:
		return EquipSlotLegs
	case ItemTypeArmorHelm:
		return EquipSlotHead
	case ItemTypeArmorChest:
		return EquipSlotChest
	}
	return EquipSlotUnknown
}

func (i *Item) GetVisualEquipSlot() EquipVisualSlot {
	switch i.typ {
	case ItemTypeArmorGloves:
		return EquipVisualSlotGloves
	case ItemTypeArmorBoots:
		return EquipVisualSlotBoots
	case ItemTypeArmorLegs:
		return EquipVisualSlotLegs
	case ItemTypeArmorHelm:
		return EquipVisualSlotHead
	case ItemTypeArmorChest:
		return EquipVisualSlotChest
	case ItemTypeWand, ItemTypeSword, ItemTypeStaff, ItemTypeHammer, ItemTypeLongbow:
		return EquipVisualSlotRightHand
	case ItemTypeShield:
		return EquipVisualSlotLeftHand
	}
	return EquipVisualSlotUnknown
}

type ItemType int

const (
	ItemTypeBag         ItemType = 3
	ItemTypeUsable      ItemType = 9
	ItemTypeCostume     ItemType = 44
	ItemTypeStaff       ItemType = 26
	ItemTypeLongbow     ItemType = 5
	ItemTypeSword       ItemType = 27
	ItemTypeHammer      ItemType = 15
	ItemTypeArmorChest  ItemType = 7
	ItemTypeArmorLegs   ItemType = 19
	ItemTypeArmorBoots  ItemType = 4
	ItemTypeArmorGloves ItemType = 13
	ItemTypeArmorHelm   ItemType = 16
	ItemTypeShield      ItemType = 24
	ItemTypeWand        ItemType = 23
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

type EquipSlot int

const (
	EquipSlotRightHand EquipSlot = iota
	EquipSlotLeftHand
	EquipSlotChest
	EquipSlotLegs
	EquipSlotHead
	EquipSlotBoots
	EquipSlotGloves
	EquipSlotCostumeBody
	EquipSlotCostumeHead
	EquipSlotUnknown
)

type EquipVisualSlot int

const (
	EquipVisualSlotUnknown   EquipVisualSlot = -1
	EquipVisualSlotRightHand EquipVisualSlot = 1
	EquipVisualSlotLeftHand  EquipVisualSlot = 0
	EquipVisualSlotLegs      EquipVisualSlot = 4
	EquipVisualSlotChest     EquipVisualSlot = 2
	EquipVisualSlotGloves    EquipVisualSlot = 5
	EquipVisualSlotHead      EquipVisualSlot = 6
	EquipVisualSlotBoots     EquipVisualSlot = 3
)

type ItemId int

const (
	ItemBackpack            ItemId = 32
	ItemStarterBow          ItemId = 447
	ItemStarterHammer       ItemId = 1699
	ItemStarterHolyRod      ItemId = 2787
	ItemStarterTruncheon    ItemId = 2694
	ItemStarterCane         ItemId = 2652
	ItemStarterElementalRod ItemId = 2742

	ItemSummoningStone          ItemId = 30847
	ItemEverlastingGhostlyStaff ItemId = 729
	ItemEternalBlade            ItemId = 1045

	ItemRingmailLeggings  ItemId = 2440
	ItemRingmailHauberk   ItemId = 823
	ItemRingmailBoots     ItemId = 355
	ItemRingmailGauntlets ItemId = 1598
	ItemRecruitsCap       ItemId = 6136

	ItemRangerMask      ItemId = 6138
	ItemRawhideVest     ItemId = 887
	ItemRawhideLeggings ItemId = 2504
	ItemRawhideGloves   ItemId = 1666
	ItemRawhideBoots    ItemId = 419

	ItemRoughspunVestments ItemId = 743
	ItemRoughspunPants     ItemId = 2359
	ItemWovenSandals       ItemId = 276
	ItemRoughspunHandwraps ItemId = 1518
	ItemMonkScalpDesign    ItemId = 6126

	ItemInitiatesTunic         ItemId = 630
	ItemInitiatesLeggings      ItemId = 2247
	ItemInitiatesBoots         ItemId = 163
	ItemInitiatesGloves        ItemId = 1405
	ItemNecromancerScarPattern ItemId = 6072

	ItemDilettantesAttire   ItemId = 562
	ItemDilettantesHose     ItemId = 2184
	ItemDilettantesFootwear ItemId = 95
	ItemDilettantesGloves   ItemId = 1341
	ItemMesmerMask          ItemId = 6062

	ItemApprenticesRobes    ItemId = 688
	ItemApprenticesLeggings ItemId = 2305
	ItemApprenticesShoes    ItemId = 221
	ItemApprenticesGloves   ItemId = 1463
	ItemThirdEye            ItemId = 6110

	ItemEternalShield ItemId = 0xcafe
)

var itemDefinitions map[ItemId]Item = map[ItemId]Item{
	ItemBackpack: {"Backpack", ItemTypeBag, 5, 0x1b536, RarityWhite, "21a8 d157 b58f 166f", []int{
		HoldsItems(20),
	}},
	// Starter weapons
	ItemStarterBow: {"Starter Bow", ItemTypeLongbow, 1000, 0x24a4, RarityWhite, "223f 87c0 c25c 378e", []int{
		//TwoHandedBow(3),
		//DamageRange(1, 3),
	}},
	ItemStarterHammer: {"StarterHammer", ItemTypeHammer, 0, 0x9B60, RarityWhite, "2455 b7fb ca41 458f", []int{
		//DamageRange(1, 3),
	}},
	ItemStarterHolyRod:      {"Holy Rod", ItemTypeWand, 0, 0x800172B8, RarityWhite, "2622 eab4 a142 18f5", []int{}},
	ItemStarterTruncheon:    {"Starter Truncheon", ItemTypeWand, 0, 0x800172D8, RarityWhite, "25d7 99f3 a510 0fb0", []int{}},
	ItemStarterCane:         {"Starter Cane", ItemTypeWand, 0, 0x80016D65, RarityWhite, "25b3 bbd8 9094 5334", []int{}},
	ItemStarterElementalRod: {"Starter Elemental Rod", ItemTypeWand, 0, 0x800172D9, RarityWhite, "25fd c833 b54c 4668", []int{}},

	// Warrior armor
	ItemRingmailLeggings:  {"Ringmail Leggings", ItemTypeArmorLegs, 0, 0x5E, RarityWhite, "2511 f291 b7db 6de4", []int{}},
	ItemRingmailHauberk:   {"Ringmail Hauberk", ItemTypeArmorChest, 0, 0x5B, RarityWhite, "22a3 fb75 a867 466d", []int{}},
	ItemRingmailBoots:     {"Ringmail Boots", ItemTypeArmorBoots, 0, 0x5A, RarityWhite, "21ee bde3 8507 57b3", []int{}},
	ItemRingmailGauntlets: {"Ringmail Gauntlets", ItemTypeArmorGloves, 0, 0x5C, RarityWhite, "2427 e062 c723 4ad2", []int{}},
	ItemRecruitsCap:       {"Recruit's Cap", ItemTypeArmorHelm, 0, 0x5D, RarityWhite, "24a3 b448 ff37 29f5", []int{}},

	// Ranger armor
	ItemRangerMask:      {"Ranger Mask", ItemTypeArmorHelm, 0, 0x8f, RarityWhite, "24b1 a91f deef 1c00", []int{}},
	ItemRawhideVest:     {"Rawhide Vest", ItemTypeArmorChest, 0, 0x95, RarityWhite, "22af fcf5 ca85 3fd4", []int{}},
	ItemRawhideLeggings: {"Rawhide Leggings", ItemTypeArmorLegs, 0, 0x97, RarityWhite, "251e 85e2 d2b5 3e2c", []int{}},
	ItemRawhideGloves:   {"Rawhide Gloves", ItemTypeArmorGloves, 0, 0x96, RarityWhite, "2434 87b4 f140 6684", []int{}},
	ItemRawhideBoots:    {"Rawhide Boots", ItemTypeArmorBoots, 0, 0x94, RarityWhite, "21fb 8ae6 a6be 77e7", []int{}},

	// Monk armor
	ItemRoughspunVestments: {"Roughspun Vestments", ItemTypeArmorChest, 0, 0xE7, RarityWhite, "228f fbde 8c53 5a69", []int{}},
	ItemRoughspunPants:     {"Roughspun Vestments", ItemTypeArmorLegs, 0, 0xE9, RarityWhite, "24fe e08e 814b 307d", []int{}},
	ItemWovenSandals:       {"Woven Sandals", ItemTypeArmorBoots, 0, 0xE6, RarityWhite, "21da d350 c1e2 0677", []int{}},
	ItemRoughspunHandwraps: {"Roughspun Handwraps", ItemTypeArmorGloves, 0, 0xE8, RarityWhite, "2414 d526 9ed7 5b6a", []int{}},
	ItemMonkScalpDesign:    {"Monk Scalp Design", ItemTypeArmorHelm, 0, 0xF5, RarityWhite, "2490 ac3d f393 40b7", []int{}},

	// Necromancer armor
	ItemInitiatesTunic:         {"Initiate's Tunic", ItemTypeArmorChest, 0, 0x140, RarityWhite, "227b de01 a012 24d7", []int{}},
	ItemInitiatesLeggings:      {"Initiate's Leggings", ItemTypeArmorLegs, 0, 0x142, RarityWhite, "24eb a90e c0d6 7016", []int{}},
	ItemInitiatesBoots:         {"Initiate's Boots", ItemTypeArmorBoots, 0, 0x13f, RarityWhite, "21c4 b527 91ff 4046", []int{}},
	ItemInitiatesGloves:        {"Initiate's Gloves", ItemTypeArmorGloves, 0, 0x141, RarityWhite, "23ff 9db1 bb1c 50b9", []int{}},
	ItemNecromancerScarPattern: {"Necromancer Scar Pattern", ItemTypeArmorHelm, 0, 0x158, RarityWhite, "2472 a63c fa08 5d34", []int{}},

	// Mesmer armor
	ItemDilettantesAttire:   {"Dilettante's Attire", ItemTypeArmorChest, 0, 0x19b, RarityWhite, "226d d6de 874c 3180", []int{}},
	ItemDilettantesHose:     {"Dilettante's Hose", ItemTypeArmorLegs, 0, 0x19d, RarityWhite, "24de c3f9 a893 676e", []int{}},
	ItemDilettantesFootwear: {"Dilettante's Footwear", ItemTypeArmorBoots, 0, 0x19a, RarityWhite, "21b7 c6a3 d47c 0727", []int{}},
	ItemDilettantesGloves:   {"Dilettante's Gloves", ItemTypeArmorGloves, 0, 0x19c, RarityWhite, "23f2 cc69 fe48 7fc1", []int{}},
	ItemMesmerMask:          {"Mesmer Mask", ItemTypeArmorHelm, 0, 0x196, RarityWhite, "2466 d37c e4ec 5d14", []int{}},

	// Elementalist armor
	ItemApprenticesRobes:    {"Apprentice's Robes", ItemTypeArmorChest, 0, 0x1f1, RarityWhite, "2287 ed03 ff8d 6062", []int{}},
	ItemApprenticesLeggings: {"Apprentice's Leggings", ItemTypeArmorLegs, 0, 0x1f3, RarityWhite, "24f6 dd72 fbc8 2bfd", []int{}},
	ItemApprenticesShoes:    {"Apprentice's Shoes", ItemTypeArmorBoots, 0, 0x1f0, RarityWhite, "21d1 d82d f170 5cb9", []int{}},
	ItemApprenticesGloves:   {"Apprentice's Gloves", ItemTypeArmorGloves, 0, 0x1f2, RarityWhite, "240b efc3 8faf 5560", []int{}},
	ItemThirdEye:            {"Third Eye", ItemTypeArmorHelm, 0, 0x8000D187, RarityWhite, "8102 0a07", []int{}},

	// Misc, non-presearing
	ItemSummoningStone: {"Summoning Stone", ItemTypeUsable, 0, 0x54992, RarityPurple, "8102 4674 F109 F330 2937", []int{
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
	ItemEternalBlade: {"Eternal Blade", ItemTypeSword, 1000, 0x17B20, RarityGold, "", []int{
		AttributeRequirement(20, 8),
		DamageType(DamageTypeSlashing),
		DamageRange(11, 22),
		EnchantDuration(20),
	}},
	ItemEternalShield: {"Eternal Shield", ItemTypeShield, 0, 0x21406, RarityGold, "", []int{}},
}

var DefaultEquipmentWarrior = []ItemId{
	ItemRecruitsCap,
	ItemRingmailHauberk,
	ItemRingmailLeggings,
	ItemRingmailGauntlets,
	ItemRingmailBoots,
	ItemStarterHammer,
}

var DefaultEquipmentRanger = []ItemId{
	ItemRangerMask,
	ItemRawhideVest,
	ItemRawhideLeggings,
	ItemRawhideGloves,
	ItemRawhideBoots,
	ItemStarterBow,
}

var DefaultEquipmentMonk = []ItemId{
	ItemMonkScalpDesign,
	ItemRoughspunVestments,
	ItemRoughspunPants,
	ItemRoughspunHandwraps,
	ItemWovenSandals,
	ItemStarterHolyRod,
}

var DefaultEquipmentNecromancer = []ItemId{
	ItemNecromancerScarPattern,
	ItemInitiatesTunic,
	ItemInitiatesLeggings,
	ItemInitiatesGloves,
	ItemInitiatesBoots,
	ItemStarterTruncheon,
}

var DefaultEquipmentMesmer = []ItemId{
	ItemMesmerMask,
	ItemDilettantesAttire,
	ItemDilettantesHose,
	ItemDilettantesGloves,
	ItemDilettantesFootwear,
	ItemStarterCane,
}

var DefaultEquipmentElementalist = []ItemId{
	ItemThirdEye,
	ItemApprenticesRobes,
	ItemApprenticesLeggings,
	ItemApprenticesGloves,
	ItemApprenticesShoes,
	ItemStarterElementalRod,
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
	if i.typ == ItemTypeArmorLegs || i.typ == ItemTypeArmorChest || i.typ == ItemTypeArmorHelm || i.typ == ItemTypeArmorGloves || i.typ == ItemTypeArmorBoots {
		flags |= 1 << 2
	}
	flags |= GetRarityFlag(i.rarity)
	if i.Name() == "Third Eye" {
		return 0x20001202
	}
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

func (i *Item) EncodeName() []byte {
	if i.encName != "" {
		return convertEncName(i.EncName())
	}
	// Fallback: directly send the name to the client.
	units := []uint16{0x0108, 0x0107}
	out := make([]byte, 0)
	units = append(units, utf16.Encode([]rune(i.Name()))...)
	units = append(units, 0x0001)
	for _, unit := range units {
		out = append(out, byte(unit&0xff), byte(unit>>8))
	}
	return out
}

func convertEncName(in string) []byte {
	// "2d9e f878 bdbf 12e7"
	conv := []byte{}
	fields := strings.Fields(in)
	for _, word := range fields {
		// Parse the 4-digit hex word into a uint16
		val, err := strconv.ParseUint(word, 16, 16)
		if err != nil {
			panic(fmt.Errorf("invalid hex word %q: %w", word, err))
		}
		conv = append(conv, byte(val&0xff), byte(val>>8))
	}
	return conv
}
