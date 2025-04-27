package db

import (
	"crypto/rand"
	"fmt"
)

type Account struct {
	ID         uint64 `gorm:"primaryKey"`
	UUID       []byte `gorm:"type:binary(16);unique"` // Fixed-length 16-byte UUID field
	Email      string `gorm:"unique"`
	Password   string
	Characters []Character `gorm:"foreignKey:AccountID"` // One-to-many relation
}

type Character struct {
	ID                  uint64 `gorm:"primaryKey"`
	Level               uint8  `gorm:"default:1"`
	ProfessionPrimary   uint8  `gorm:"default:1"` // Just in case!
	ProfessionSecondary uint8  `gorm:"default:0"`
	UUID                []byte `gorm:"type:binary(16);unique"`
	LastOutpostID       uint16 `gorm:"default:165"` // Just in case!
	Appearance          []byte `gorm:"type:binary(8)"`
	EquipmentData       []byte
	AccountID           uint64  // Foreign key to Account
	Account             Account `gorm:"foreignKey:AccountID"` // ForeignKey relation
	Name                string
}

func autoMigrate() (err error) {
	// AutoMigrate models to create tables (including foreign key)
	err = database.AutoMigrate(&Account{}, &Character{})
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

func maybeBootstrap() (err error) {
	var count int64
	database.Model(&Account{}).Count(&count)
	if count > 0 {
		return
	}
	// Set initial data as there were no accounts
	rootAccount := Account{
		Email:    "root@localhost",
		Password: "p",
		UUID:     randUuid(),
	}
	database.Create(&rootAccount)
	// One character
	rootChar := Character{
		AccountID:  rootAccount.ID,
		UUID:       randUuid(),
		Name:       "Default Char",
		Appearance: []byte{0x11, 0x18, 0x21, 0x06, 0x00, 0x00, 0x00, 0x00},
		EquipmentData: []byte{
			0x11, 0x40, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x8f, 0x00, 0x02, 0x00, 0x07, 0x95, 0x00,
			0x02, 0x00, 0x07, 0x96, 0x00, 0x02, 0x00, 0x07, 0x97, 0x00, 0x02, 0x00, 0x07, 0x94, 0x00, 0x02,
			0x00, 0x07},
	}
	database.Create(&rootChar)
	return
}
