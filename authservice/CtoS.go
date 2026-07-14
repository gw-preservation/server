//lint:file-ignore U1000 Fields are not unused
//go:generate go run ../cmd/codegen/main.go c2s fmt errors
//go:generate go fmt

package AuthService

// opcode: 0x800a
type SetActiveCharacter struct {
	reqNumber int // wire:uint32
	charName  string
}

// opcode: 0x8029
type LoginCharacter struct {
	reqNumber int // wire:uint32
	unk1      int // wire:uint32
	mapId     int // wire:uint32
	unk2      int // wire:uint32
	unk3      int // wire:uint32
	unk4      int // wire:uint32
}

// opcode: 0x801c
type AddAccessKey struct {
	reqNumber int // wire:uint32
	key       string
}

// opcode: 0x8002
type ClientHashInfo struct {
	clientVersion int    // wire:uint32
	unkHash       []byte // len:16
}

// opcode: 0x0400
type ClientVersionInfo struct {
	skip          int // wire:uint32
	clientVersion int // wire:uint32
	unk2          int // wire:uint32
	unk3          int // wire:uint32
}

// opcode: 0x8000
type Unknown8000 struct {
	unk1 int // wire:uint32
}

// opcode: 0x8001
type ComputerInfo struct {
	userName     string
	computerName string
}

// opcode: 0x8023
type Unknown8023 struct {
	unk1 int // wire:uint32
}

// opcode: 0x8038
type GetAccountInfo struct {
	reqNumber                  int    // wire:uint32
	uuid1                      []byte // len:16
	gameTokenFromPortalService []byte // len:16
	unk1                       string
}

// opcode: 0x8035
type AskServerResponse struct {
	reqNumber int // wire:uint32
}

// opcode: 0x8016
type LanguageInfo struct {
	languageCode int // wire:uint32
}

// opcode: 0x800d
type Disconnect struct {
	errorCode int // wire:uint32
}

// opcode: 0x800e
type SetPlayerOnlineVisibilityStatus struct {
	visibility int // wire:uint32
}

// opcode: 0x4200
type ClientSeed struct {
	skip      int    // wire:uint16
	seedBytes []byte // len:64
}

// opcode: 0x8021
type UpdateSettingsLength struct {
	reqNumber int // wire:uint32
	unk2      int // wire:uint32
}

// opcode: 0x8020
type UpdateSettings struct {
	unk1     int    // wire:uint32
	settings []byte // wire:VarByte
}

// opcode: 0x8007
type DeleteCharacter struct {
	reqNumber int // wire:uint32
	name      string
}
