package main

import (
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite" // actual pure Go SQLite driver
)

type Book struct {
	ID     uint
	Title  string
	Author string
}

func main() {
	fmt.Printf("alive.\n")
	// Use the "sqlite" wrapper with DSN prefix "file:"
	db, err := gorm.Open(sqlite.Dialector{
		DSN:        "file:books.db?mode=rwc",
		DriverName: "sqlite", // must match the registered driver name
	}, &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Migrate schema and use it like normal
	db.AutoMigrate(&Book{})
	db.Create(&Book{Title: "1984", Author: "George Orwell"})
}
