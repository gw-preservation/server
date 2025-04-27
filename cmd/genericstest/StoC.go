//go:generate go run ../codegen/main.go s2c
//go:generate go fmt

package main

// opcode: 0x0003
type RequestResponse struct {
	reqNumber    int // wire:uint32
	responseCode int // wire:uint32
}

// opcode: 0x0007
type CharacterSummary struct {
	reqNumber  int // wire:uint32
	charUUID   []byte
	unk1       int // wire:uint32
	charName   string
	summaryLen int // wire:uint16
	summary    []byte
}

// opcode: 0x0014
type AccountExtraInfoStart struct {
	reqNumber int // wire:uint32
	unk1      int // wire:uint32
}

// opcode: 0x0011
type AccountExtraInfo struct {
	reqNumber          int // wire:uint32
	territoryCode      int // wire:uint32
	languageCode       int // wire:uint32
	unk1               []byte
	unk2               []byte
	accountUUID        []byte
	activeCharUUID     []byte
	unk3               int // wire:uint32
	entitlementsLength int // wire:uint16
	entitlements       []byte
	eulaByte           int // wire:uint8
	unk4               int // wire:uint32
}

// opcode: 0x0016
type AccountBinaryInfo struct {
	reqNumber   int //wire:uint32
	blockLength int //wire:uint16
	blockBytes  []byte
}

// opcode: 0x1601
type ServerSeed struct {
	xoredRandomBytes []byte
}

// opcode: 0x0001
type SessionSaltInfo struct {
	salt int //wire:uint32
	unk1 int //wire:uint32
}

// opcode: 0x0009
type InstanceServerInfo struct {
	reqNumber  int //wire:uint32
	worldHash  int //wire:uint32
	mapId      int //wire:uint32
	socketData []byte
	playerHash int //wire:uint32
}
