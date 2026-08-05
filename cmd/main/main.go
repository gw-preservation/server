package main

import (
	"flag"
	gw1 "gw1/server"
	"gw1/server/authservice"
	"gw1/server/db"
	"gw1/server/gameservice"
	"gw1/server/portalservice"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
)

const serverIP = "192.168.1.124"

var gwDatPath = flag.String("gwdat", "./Gw.dat", "path to the Guild Wars Gw.dat archive (pathing data)")
var logLevel = flag.String("log", "info", "log level: debug, info, warn, error, disabled")

func main() {
	flag.Parse()
	switch strings.ToLower(*logLevel) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "disabled":
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	if err := db.Initialize(); err != nil {
		panic(err)
	}
	if err := gameservice.InitializeInstances(*gwDatPath); err != nil {
		panic(err)
	}
	ip := net.ParseIP(serverIP).To4()
	copy(authservice.ServerIP[:], ip)
	copy(gameservice.ServerIP[:], ip)
	srv := gw1.NewTCPServer()

	// Set up signal channel
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("Received signal: %s, shutting down...", sig)
		portalservice.StopCleanup()
		gameservice.StopCleanup()
		db.Close()
		os.Exit(0)
	}()

	srv.Serve()
}
