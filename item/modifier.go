package Item

func buildModifierInt(id int, val1 int, val2 int) int {
	return id<<16 | val1<<8 | val2
}

func HalfSkillRecharge(chance int) int {
	return buildModifierInt(0x23a8, chance, 0)
}

func HoldsItems(count int) int {
	return buildModifierInt(0x2448, count, 0)
}

func TwoHandedBow(unk int) int {
	return buildModifierInt(0x2618, unk, 0)
}

func SummonStoneModifier(unk int) int {
	return buildModifierInt(0x2788, 0, unk)
}

func AttributeRequirement(attr int, req int) int {
	return buildModifierInt(0x2798, attr, req)
}

func DamageType(typ int) int {
	return buildModifierInt(0x24b8, typ, 0)
}

func EnchantDuration(extraDur int) int {
	return buildModifierInt(0x22b8, 0, extraDur)
}

func ExtraEnergyHealthOver(extra int, threshold int) int {
	return buildModifierInt(0x2308, threshold, extra)
}

func ExtraEnergy(amount int) int {
	return buildModifierInt(0x62c8, amount, 0)
}

func DamageRange(min int, max int) int {
	return buildModifierInt(0xa7a8, max, min)
}
