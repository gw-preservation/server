#!/bin/sh
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o server cmd/main/main.go
