package main

import (
	"github.com/rs/zerolog"
)

func zeroLog() {

	rootLogger := zerolog.New(zerolog.NewConsoleWriter())

	rootLogger = rootLogger.Level(zerolog.InfoLevel)
	rootLogger = rootLogger.With().Timestamp().Logger()

	mydata := []byte{0x0c, 0xca, 0xfe}
	clientLogger := rootLogger.Level(zerolog.DebugLevel).With().Str("type", "client").Logger()
	clientLogger.Error().Hex("data", mydata).Msgf("Hi %s", "world")

	authLogger := rootLogger.With().Str("type", "auth").Str("opcode", "808f").Logger()
	authLogger = authLogger.Level(zerolog.DebugLevel)
	authLogger.Debug().Hex("data", mydata).Msg("")
}

func main() {
	zeroLog()
}
