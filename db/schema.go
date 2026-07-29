package db

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	Item "gw1/server/item"
)

type Account struct {
	ID           uint64 `gorm:"primaryKey"`
	UUID         []byte `gorm:"type:binary(16);unique"` // Fixed-length 16-byte UUID field
	Email        string `gorm:"unique"`
	Password     string
	PasswordSalt []byte      `gorm:"type:binary(8)"`
	Characters   []Character `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE;"` // One-to-many relation, cascade delete
}

type Character struct {
	ID                  uint64  `gorm:"primaryKey"`
	Level               uint8   `gorm:"default:1"`
	ProfessionPrimary   uint8   `gorm:"default:1"` // Just in case!
	ProfessionSecondary uint8   `gorm:"default:0"`
	UUID                []byte  `gorm:"type:binary(16);unique"`
	LastOutpostID       uint16  `gorm:"default:148"` // Just in case!
	AppearanceBits      uint32  `gorm:"default:0"`
	AccountID           uint64  // Foreign key to Account
	Account             Account `gorm:"foreignKey:AccountID"` // ForeignKey relation
	Name                string
	XP                  uint32 `gorm:"default:0"`
	Bags                []Bag  `gorm:"foreignKey:CharacterID;constraint:OnDelete:CASCADE;"` // One-to-many relation, cascade delete
}

type Bag struct {
	ID          uint64 `gorm:"primaryKey"`
	Capacity    uint8
	CharacterID uint64 // Foreign key to Character
	Type        uint8  // Bag=1, Equipped=2
	Slots       []Slot `gorm:"foreignKey:BagID"` // One-to-many relationship with Slot
}

type Slot struct {
	ID            uint64         `gorm:"primaryKey"`
	ItemID        uint32         // Set to 0 for unused slot!
	BagID         uint64         // Foreign key to Bag
	ItemQuantity  uint32         `gorm:"default:1"` // Just in case!
	ItemModifiers ModifiersArray `gorm:"type:json"`
	Dye1          uint8
}

type ModifiersArray []uint32

// Scan implements the Scanner interface for reading from the database
func (u *ModifiersArray) Scan(value interface{}) error {
	// Try to convert the value to a JSON string
	b, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan Uint32Array")
	}

	// Unmarshal the JSON into the slice
	return json.Unmarshal(b, &u)
}

// Value implements the Valuer interface for writing to the database
func (u ModifiersArray) Value() (driver.Value, error) {
	// Marshal the slice to JSON before storing in the database
	return json.Marshal(u)
}

func autoMigrate() (err error) {
	// AutoMigrate models to create tables (including foreign key)
	err = database.AutoMigrate(&Account{}, &Character{}, &Bag{}, &Slot{})
	return err
}

func UUIDStr(uuid []byte) string {
	// "00010203-0405-0607-0809-0A0B0C0D0E0F"
	return fmt.Sprintf(
		"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		uuid[0], uuid[1], uuid[2], uuid[3],
		uuid[4], uuid[5],
		uuid[6], uuid[7],
		uuid[8], uuid[9],
		uuid[10], uuid[11], uuid[12], uuid[13], uuid[14], uuid[15],
	)
}

func UUIDStrSwapped(uuid []byte) string {
	// "00010203-0405-0607-0809-0A0B0C0D0E0F"
	return fmt.Sprintf(
		"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		uuid[3], uuid[2], uuid[1], uuid[0],
		uuid[5], uuid[4],
		uuid[7], uuid[6],
		uuid[8], uuid[9],
		uuid[10], uuid[11], uuid[12], uuid[13], uuid[14], uuid[15],
	)
}

func randUuid() []byte {
	res := make([]byte, 16)
	rand.Read(res)
	return res
}

func randSalt() []byte {
	res := make([]byte, 8)
	rand.Read(res)
	return res
}

func CreateDefaultBagsAndItems(characterId uint64, primaryProfession int, equipDyes [7]int) []Bag {
	inventory := NewDbBag(characterId, 20, 1)
	equipment := NewDbBag(characterId, 9, 2)

	inventory.Slots[0] = Slot{
		BagID:        inventory.ID,
		ItemID:       uint32(Item.ItemEverlastingGhostlyStaff),
		ItemQuantity: 1,
	}
	inventory.Slots[1] = Slot{
		BagID:        inventory.ID,
		ItemID:       uint32(Item.ItemSummoningStone),
		ItemQuantity: 1,
	}

	var equips []Item.ItemId
	switch primaryProfession {
	case 1:
		equips = Item.DefaultEquipmentWarrior
		// Test warrior items
		equipment.Slots[0] = Slot{
			BagID:        equipment.ID,
			ItemID:       uint32(Item.ItemEternalBlade),
			ItemQuantity: 1,
		}
		equipment.Slots[1] = Slot{
			BagID:        equipment.ID,
			ItemID:       uint32(Item.ItemEternalShield),
			ItemQuantity: 1,
		}
	case 2:
		equips = Item.DefaultEquipmentRanger
	case 3:
		equips = Item.DefaultEquipmentMonk
	case 4:
		equips = Item.DefaultEquipmentNecromancer
	case 5:
		equips = Item.DefaultEquipmentMesmer
	case 6:
		equips = Item.DefaultEquipmentElementalist
	}
	for _, itemid := range equips {
		itm := Item.GetItemDefinitionById(itemid)
		eSlot := itm.GetEquipSlot()
		equipment.Slots[eSlot] = Slot{
			BagID:        equipment.ID,
			ItemID:       uint32(itemid),
			ItemQuantity: 1,
			Dye1:         uint8(equipDyes[eSlot]),
		}
	}

	return []Bag{inventory, equipment}
}

func maybeBootstrap() (err error) {
	var count int64
	database.Model(&Account{}).Count(&count)
	if count > 0 {
		return
	}
	// Set initial data as there were no accounts
	rootAccount := Account{
		Email:        "root@localhost",
		Password:     "p",
		PasswordSalt: randSalt(),
		UUID:         randUuid(),
	}
	database.Create(&rootAccount)
	// One character
	primaryProfession := uint8(4)
	AddDbChar(rootAccount.ID, "Default Char", int(primaryProfession), 0x0744943b, CreateDefaultBagsAndItems(0, int(primaryProfession), [7]int{}))

	// Make an alt account
	altAccount := Account{
		Email:        "alt@localhost",
		Password:     "p",
		PasswordSalt: randSalt(),
		UUID:         randUuid(),
	}
	database.Create(&altAccount)
	primaryProfession = uint8(2)
	AddDbChar(altAccount.ID, "Alt Char 1", int(primaryProfession), 0x042094e6, CreateDefaultBagsAndItems(0, int(primaryProfession), [7]int{}))

	// Make a second alt account
	altAccount2 := Account{
		Email:        "alt2@localhost",
		Password:     "p",
		PasswordSalt: randSalt(),
		UUID:         randUuid(),
	}
	database.Create(&altAccount2)
	primaryProfession = uint8(5)
	AddDbChar(altAccount2.ID, "Alt Char 2", int(primaryProfession), 0x045171b5, CreateDefaultBagsAndItems(0, int(primaryProfession), [7]int{}))
	return
}
