#!/bin/sh
killall entr main
find . -name "*.go" | entr -c -r go run cmd/main/main.go
