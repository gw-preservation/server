package main

import (
	gw1 "gw1/server"
	"gw1/server/db"
	GameService "gw1/server/gameservice"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := db.Initialize(); err != nil {
		panic(err)
	}
	if err := GameService.LoadInstanceDefinitionsFromDisk(); err != nil {
		panic(err)
	}
	srv := gw1.NewTCPServer()

	// Set up signal channel
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("Received signal: %s, shutting down...", sig)
		db.Close() // clean shutdown
		os.Exit(0)
	}()

	srv.Serve()
}
