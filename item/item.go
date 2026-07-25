package Item

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type dbModifierRef struct {
	Name string `json:"name"`
	Val1 int    `json:"val1"`
	Val2 int    `json:"val2"`
}

type dbItem struct {
	Name              string          `json:"name"`
	EncName           string          `json:"enc_name"`
	Type              string          `json:"type"`
	ModelFileId       HexInt          `json:"model_file_id"`
	BaseMerchantValue int             `json:"merchant_value"`
	InherentModifiers []dbModifierRef `json:"inherent_modifiers"`
	Rarity            string          `json:"rarity"`
}

// DamageType modifier:
// 1=Piercing
// 2=Slashing
// 3=Cold
// 4=Lightning
// 5=Fire
// 6=Chaos
// 7=Dark
// 8=Holy
// 9=Nature
// 10=Sacrifice
// 11=Earth
// 12=Generic
// 13=Dark

func (i dbItem) MarshalModifiers() (out []uint32) {
	for _, mod := range i.InherentModifiers {
		modId, ok := itemDefinitions.Modifiers[mod.Name]
		if !ok {
			panic(fmt.Errorf("unable to marshal modifier '%s': no definition", mod.Name))
		}
		completeModId := uint32(modId)<<16 | uint32(mod.Val1)<<8 | uint32(mod.Val2)
		out = append(out, completeModId)
	}
	return
}

type HexInt int

func (h *HexInt) UnmarshalJSON(data []byte) error {
	// Remove quotes from the string (JSON string is quoted)
	s := strings.Trim(string(data), "\"")

	// Use strconv to parse hex (0x prefix is allowed)
	val, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return err
	}

	*h = HexInt(val)
	return nil
}

var itemDefinitions = struct {
	Items     map[int]*dbItem   `json:"items"`
	Modifiers map[string]HexInt `json:"modifiers"`
}{
	Items:     make(map[int]*dbItem, 0),
	Modifiers: make(map[string]HexInt, 0),
}

func LoadItemDefinitionsFromDisk() error {
	file, err := os.Open("item/items.json")
	if err != nil {
		return fmt.Errorf("failed to load item definitions file: %w", err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&itemDefinitions); err != nil {
		return fmt.Errorf("failed to parse item definitions: %w", err)
	}

	// Fix up ModelFileID
	for _, key := range itemDefinitions.Items {
		key.ModelFileId |= 0x80000000
	}
	return nil
}

func GetItemDefinitionById(fileId int) (item *dbItem) {
	var ok bool
	item, ok = itemDefinitions.Items[fileId]
	if !ok {
		panic(fmt.Sprintf("GetItemDefinitionById(%d): no definition!", fileId))
	}
	return
}

func (i *dbItem) ComputeInteractionFlags() int {
	flags := 0x22201000
	flags |= i.GetRarityFlag()
	return flags
}

func (i *dbItem) GetRarityFlag() int {
	switch i.Rarity {
	case "purple":
		return 0x400000
	case "green":
		return 0x10
	case "gold":
		return 0x20000
	}
	return 0
}
