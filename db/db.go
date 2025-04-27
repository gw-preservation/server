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
