package db

import (
	"errors"
	"fmt"

	_ "github.com/glebarez/sqlite" // actual pure Go SQLite driver
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func CharacterNameExists(name string) bool {
	var id uint64
	err := database.Model(&Character{}).
		Select("id").
		Where("name = ?", name).
		Take(&id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return false
	case err != nil:
		log.Error().Str("name", name).Err(err).Msg("unable to query whether character name exists")
		return true
	default:
		return true
	}
}

func DeleteDbChar(name string, requestedByAccId uint64) error {
	result := database.Where(
		"name = ? AND account_id = ?",
		name,
		requestedByAccId,
	).Delete(&Character{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("character not found or not owned by account")
	}

	return nil
}

func GetAccountByEmail(email string) (acc Account, ok bool) {
	err := database.Where("email = ?", email).First(&acc).Error
	if err != nil {
		ok = false
	}
	ok = true
	return
}

func GetFullAccountByEmail(email string) (acc Account, ok bool) {
	err := database.Preload("Characters").First(&acc, "email = ?", email).Error
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

func AddDbChar(forAccountId uint64, name string, primaryProfession int, appearanceBits uint32) (char Character) {
	log.Info().Uint64("forAccountId", forAccountId).Str("name", name).Int("primary", primaryProfession).Uint32("appearance", appearanceBits).Msg("NewDbChar")
	char.AccountID = forAccountId
	char.UUID = randUuid()
	char.Name = name
	char.ProfessionPrimary = uint8(primaryProfession)
	char.ProfessionSecondary = 0
	char.AppearanceBits = appearanceBits
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

	/*// give costume
	equipment.Slots[7] = Slot{
		BagID:        equipment.ID,
		ItemID:       1085,
		ItemType:     44,
		ItemQuantity: 1,
	}*/
	database.Create(&char)
	return
}

func SetLastOutpostForChar(charId uint64, outpostId uint16) error {
	result := database.Model(&Character{}).
		Where("id = ?", charId).
		Update("last_outpost_id", outpostId)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("character not found")
	}

	return nil
}
