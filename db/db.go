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
var ErrCharacterNotFound = errors.New("character not found")

var log zerolog.Logger
var database *gorm.DB

func Initialize() error {
	log = zerolog.New(zerolog.NewConsoleWriter())
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

func SetupTestDB() error {
	log = zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	var err error
	database, err = gorm.Open(sqlite.Dialector{
		DSN:        "file::memory:?cache=shared",
		DriverName: "sqlite",
	}, &gorm.Config{})
	if err != nil {
		return err
	}
	return autoMigrate()
}

func CharacterNameExists(name string) bool {
	var count int64
	err := database.Model(&Character{}).
		Where("name = ?", name).
		Count(&count).Error
	if err != nil {
		log.Error().Str("name", name).Err(err).Msg("unable to query whether character name exists")
		return true
	}
	return count > 0
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

func NewDbSlot() (slot Slot) {
	slot.ItemModifiers = make([]uint32, 0)
	return
}

func NewDbBag(forCharacterId uint64, capacity int, bagType int) (bag Bag) {
	bag.CharacterID = forCharacterId
	bag.Capacity = uint8(capacity)
	bag.Type = uint8(bagType)
	for range capacity {
		bag.Slots = append(bag.Slots, NewDbSlot())
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
		var count int64
		err := tx.Model(&Character{}).Where("name = ?", name).Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrCharacterNameTaken
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

func SaveCharacterMapTransfer(charId uint64, newMapId uint16, newBags []Bag, updateOutpost bool) error {
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
		if updateOutpost {
			result := tx.Model(&Character{}).Where("id = ?", charId).Update("last_outpost_id", newMapId)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.New("character not found")
			}
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

func RenameCharacter(oldName, newName string, accountID uint64) error {
	return database.Transaction(func(tx *gorm.DB) error {
		var char Character
		err := tx.Where("name = ? AND account_id = ?", oldName, accountID).First(&char).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCharacterNotFound
		}
		if err != nil {
			return err
		}

		if oldName != newName {
			var count int64
			err = tx.Model(&Character{}).Where("name = ?", newName).Count(&count).Error
			if err != nil {
				return err
			}
			if count > 0 {
				return ErrCharacterNameTaken
			}
		}

		return tx.Model(&char).Update("name", newName).Error
	})
}

func CreateAccountForTest(acc *Account) error {
	return database.Create(acc).Error
}
