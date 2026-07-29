package main

import (
	gw1 "gw1/server"
	AuthService "gw1/server/authservice"
	"gw1/server/db"
	GameService "gw1/server/gameservice"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const serverIP = "192.168.1.124"

func main() {
	if err := db.Initialize(); err != nil {
		panic(err)
	}
	if err := GameService.InitializeInstances(); err != nil {
		panic(err)
	}
	ip := net.ParseIP(serverIP).To4()
	copy(AuthService.ServerIP[:], ip)
	copy(GameService.ServerIP[:], ip)
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
