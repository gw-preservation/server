package db

import (
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite" // actual pure Go SQLite driver
)

var log zerolog.Logger
var database *gorm.DB

func Initialize() error {
	log = zerolog.New(zerolog.NewConsoleWriter())
	log = log.Level(zerolog.DebugLevel)
	log = log.With().Str("origin", "db").Logger()
	log = log.With().Timestamp().Logger()
	var err error
	database, err = gorm.Open(sqlite.Dialector{
		DSN:        "file:db.sqlite3?mode=rwc",
		DriverName: "sqlite", // must match the registered driver name
	}, &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := maybeBootstrap(); err != nil {
		return fmt.Errorf("maybeBootstrap: %w", err)
	}
	return nil
}

func Close() {
	if database != nil {
		d, err := database.DB()
		if err != nil {
			return
		}
		d.Close()
	}
}

func GetAccountByEmail(email string) (acc Account, ok bool) {
	err := database.Where("email = ?", email).First(&acc).Error
	if err != nil {
		ok = false
	}
	ok = true
	return
}

func GetFullAccountByID(accountId uint64) (acc Account, ok bool) {
	err := database.Preload("Characters").First(&acc, "ID = ?", accountId).Error
	if err != nil {
		ok = false
	}
	ok = true
	return
}

func GetFullAccountByUUID(accountUUID []byte) (acc Account, ok bool) {
	err := database.Preload("Characters").First(&acc, "UUID = ?", accountUUID).Error
	if err != nil {
		ok = false
	}
	ok = true
	return
}

func GetBagsForCharacterByID(characterId uint64) (bags []Bag, ok bool) {
	err := database.Where("character_id = ?", characterId).Preload("Slots").Find(&bags).Error
	if err != nil {
		ok = false
	}
	ok = true
	return
}

func NewDbSlot(forBagId uint64) (slot Slot) {
	slot.BagID = forBagId
	slot.ItemModifiers = make([]uint32, 0)
	return
}

func NewDbBag(forCharacterId uint64, capacity int, bagType int) (bag Bag) {
	bag.CharacterID = forCharacterId
	bag.Capacity = uint8(capacity)
	bag.Type = uint8(bagType)
	for range capacity {
		bag.Slots = append(bag.Slots, NewDbSlot(bag.ID))
	}
	return
}

func NewDbChar(forAccountId uint64, name string, primaryProfession int, appearanceBits []byte) (char Character) {
	char.AccountID = forAccountId
	char.UUID = randUuid()
	char.Name = name
	char.ProfessionPrimary = uint8(primaryProfession)
	char.ProfessionSecondary = 0
	char.Appearance = appearanceBits
	char.EquipmentData = []byte{
		0x11, 0x40, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x8f, 0x00, 0x02, 0x00, 0x07, 0x95, 0x00,
		0x02, 0x00, 0x07, 0x96, 0x00, 0x02, 0x00, 0x07, 0x97, 0x00, 0x02, 0x00, 0x07, 0x94, 0x00, 0x02,
		0x00, 0x07,
	}
	// Give it an inventory bag
	inventory := NewDbBag(char.ID, 20, 1)
	equipment := NewDbBag(char.ID, 9, 2) // TODO: why 9 and not 8?
	char.Bags = append(char.Bags, inventory, equipment)

	// give some test items
	inventory.Slots[0] = Slot{
		BagID:        inventory.ID,
		ItemID:       447,
		ItemType:     3,
		ItemQuantity: 1,
	}
	inventory.Slots[1] = Slot{
		BagID:        inventory.ID,
		ItemID:       30847,
		ItemType:     9,
		ItemQuantity: 1,
	}

	// give costume
	equipment.Slots[7] = Slot{
		BagID:        equipment.ID,
		ItemID:       1085,
		ItemType:     44,
		ItemQuantity: 1,
	}
	return
}
