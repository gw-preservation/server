#!/bin/sh
go build -ldflags="-s -w" -o server cmd/main/main.go