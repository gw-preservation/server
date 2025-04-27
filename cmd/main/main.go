package main

import (
	gw1 "gw1/server"
	GameService "gw1/server/gameservice"

	"github.com/charmbracelet/log"
)

var logger = log.WithPrefix("main")

func main() {
	logger.SetLevel(log.DebugLevel)
	GameService.LoadInstanceDefinitionsFromDisk()
	srv := gw1.NewTCPServer()
	srv.Serve()
}
