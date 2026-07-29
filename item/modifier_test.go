package Item

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildModifierInt(t *testing.T) {
	result := buildModifierInt(0x23a8, 5, 10)
	expected := 0x23a8<<16 | 5<<8 | 10
	assert.Equal(t, expected, result)
}

func TestBuildModifierInt_Zeros(t *testing.T) {
	result := buildModifierInt(0, 0, 0)
	assert.Equal(t, 0, result)
}

func TestHalfSkillRecharge(t *testing.T) {
	result := HalfSkillRecharge(20)
	expected := 0x23a8<<16 | 20<<8 | 0
	assert.Equal(t, expected, result)
}

func TestHoldsItems(t *testing.T) {
	result := HoldsItems(20)
	expected := 0x2448<<16 | 20<<8 | 0
	assert.Equal(t, expected, result)
}

func TestTwoHandedBow(t *testing.T) {
	result := TwoHandedBow(3)
	expected := 0x2618<<16 | 3<<8 | 0
	assert.Equal(t, expected, result)
}

func TestSummonStoneModifier(t *testing.T) {
	result := SummonStoneModifier(136)
	expected := 0x2788<<16 | 0<<8 | 136
	assert.Equal(t, expected, result)
}

func TestAttributeRequirement(t *testing.T) {
	result := AttributeRequirement(12, 9)
	expected := 0x2798<<16 | 12<<8 | 9
	assert.Equal(t, expected, result)
}

func TestDamageType(t *testing.T) {
	result := DamageType(DamageTypeChaos)
	expected := 0x24b8<<16 | DamageTypeChaos<<8 | 0
	assert.Equal(t, expected, result)
}

func TestEnchantDuration(t *testing.T) {
	result := EnchantDuration(20)
	expected := 0x22b8<<16 | 0<<8 | 20
	assert.Equal(t, expected, result)
}

func TestExtraEnergyHealthOver(t *testing.T) {
	result := ExtraEnergyHealthOver(5, 50)
	expected := 0x2308<<16 | 50<<8 | 5
	assert.Equal(t, expected, result)
}

func TestExtraEnergy(t *testing.T) {
	result := ExtraEnergy(15)
	expected := 0x62c8<<16 | 15<<8 | 0
	assert.Equal(t, expected, result)
}

func TestDamageRange(t *testing.T) {
	result := DamageRange(11, 22)
	expected := 0xa7a8<<16 | 22<<8 | 11
	assert.Equal(t, expected, result)
}

func TestModifierRoundTrip_ViaItemMods(t *testing.T) {
	item, err := GetItemDefinitionById(ItemEverlastingGhostlyStaff)
	require.NoError(t, err)
	mods := item.MarshalModifiers()
	assert.Len(t, mods, 7)
	assert.Equal(t, uint32(AttributeRequirement(12, 9)), mods[0])
	assert.Equal(t, uint32(DamageType(DamageTypeChaos)), mods[1])
	assert.Equal(t, uint32(HalfSkillRecharge(20)), mods[2])
	assert.Equal(t, uint32(EnchantDuration(20)), mods[3])
	assert.Equal(t, uint32(ExtraEnergyHealthOver(5, 50)), mods[4])
	assert.Equal(t, uint32(ExtraEnergy(15)), mods[5])
	assert.Equal(t, uint32(DamageRange(11, 22)), mods[6])
}
