package db

import (
	"errors"
	"fmt"

	_ "github.com/glebarez/sqlite" // actual pure Go SQLite driver
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var ErrCharacterNameTaken = errors.New("character name already taken")

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
	ok = err == nil
	return
}

func GetFullAccountByID(accountId uint64) (acc Account, ok bool) {
	err := database.Preload("Characters").First(&acc, "ID = ?", accountId).Error
	ok = err == nil
	return
}

func GetFullAccountByUUID(accountUUID []byte) (acc Account, ok bool) {
	err := database.Preload("Characters").First(&acc, "UUID = ?", accountUUID).Error
	ok = err == nil
	return
}

func GetBagsForCharacterByID(characterId uint64) (bags []Bag, ok bool) {
	err := database.Where("character_id = ?", characterId).Preload("Slots").Find(&bags).Error
	ok = err == nil
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

func AddDbChar(forAccountId uint64, name string, primaryProfession int, appearanceBits uint32, bags []Bag) (char Character, err error) {
	log.Info().Uint64("forAccountId", forAccountId).Str("name", name).Int("primary", primaryProfession).Uint32("appearance", appearanceBits).Msg("NewDbChar")
	char.AccountID = forAccountId
	char.UUID = randUuid()
	char.Name = name
	char.ProfessionPrimary = uint8(primaryProfession)
	char.ProfessionSecondary = 0
	char.AppearanceBits = appearanceBits
	char.Bags = bags
	err = database.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&char).Error
	})
	return
}

func CreateCharacter(accountID uint64, name string, primaryProfession int, appearanceBits uint32, bags []Bag) (char Character, err error) {
	err = database.Transaction(func(tx *gorm.DB) error {
		var id uint64
		err := tx.Model(&Character{}).Select("id").Where("name = ?", name).Take(&id).Error
		if err == nil {
			return ErrCharacterNameTaken
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		char.AccountID = accountID
		char.UUID = randUuid()
		char.Name = name
		char.ProfessionPrimary = uint8(primaryProfession)
		char.ProfessionSecondary = 0
		char.AppearanceBits = appearanceBits
		char.Bags = bags
		return tx.Create(&char).Error
	})
	return
}

func SaveCharacterMapTransfer(charId uint64, newMapId uint16, newBags []Bag) error {
	return database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bag_id IN (SELECT id FROM bags WHERE character_id = ?)", charId).Delete(&Slot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("character_id = ?", charId).Delete(&Bag{}).Error; err != nil {
			return err
		}
		for i := range newBags {
			newBags[i].CharacterID = charId
			if err := tx.Create(&newBags[i]).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&Character{}).Where("id = ?", charId).Update("last_outpost_id", newMapId)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("character not found")
		}
		return nil
	})
}

func ReplaceBagsForCharacter(characterId uint64, newBags []Bag) error {
	return database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bag_id IN (SELECT id FROM bags WHERE character_id = ?)", characterId).Delete(&Slot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("character_id = ?", characterId).Delete(&Bag{}).Error; err != nil {
			return err
		}
		for i := range newBags {
			newBags[i].CharacterID = characterId
			if err := tx.Create(&newBags[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
