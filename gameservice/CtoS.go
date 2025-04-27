//lint:file-ignore U1000 Fields are not unused
//go:generate go run ../cmd/codegen/main.go c2s errors fmt
//go:generate go fmt

package GameService

type VarByte []byte
type VarUTF16 []byte

// opcode: 0x808a
type CreateCharacterFinish struct {
	name       string
	appearance []byte // len:8
}

// opcode: 0x808f
type InstanceLoadRequestSync struct {
	unk1 []byte // len:16
}

// opcode: 0x8091
type Unknown8091 struct {
	unk1 VarByte
}

// opcode: 0x0500
type VerifyClientConnection struct {
	unk1          int    //wire:uint16
	clientVersion int    //wire:uint16
	unk3          int    //wire:uint16
	unk4          int    //wire:uint32
	worldHash     int    //wire:uint32
	mapId         int    //wire:uint32
	playerHash    int    //wire:uint32
	accountUUID   []byte //len:16
	characterUUID []byte //len:16
	unk5          int    //wire:uint16
	unk6          int    //wire:uint32
}

// opcode: 0x8009
type PingReply struct {
	unk1 int //wire:uint32
}

// opcode: 0x8083
type DyeEquipment struct {
	slot  int //wire:uint8
	color int //wire:uint8
}

// opcode: 0x805f
type UpdateProfessionChoice struct {
	isPvE        bool
	professionId int //wire:uint8
}

// opcode: 0x800a
type GpuInformation struct {
	unk1          []byte //len:16
	unk2          int    //wire:uint32
	unk3          int    //wire:uint32
	unk4          int    //wire:uint32
	unk5          []byte //len:12
	unk6          int    //wire:uint32
	unk7          int    //wire:uint32
	unk8          int    //wire:uint32
	unk9          int    //wire:uint32
	unk10         int    //wire:uint32
	unk11         int    //wire:uint32
	unk12         int    //wire:uint32
	driverName    string
	driverVersion string
}

// opcode: 0x4200
type ClientSeed struct {
	seed []byte //len:64
}

// opcode: 0x8063
type ChatMessage struct {
	agentId int //wire:uint32
	message string
}

// opcode: 0x803d
type MoveToPoint struct {
	x     float32
	y     float32
	plane int //wire:uint32
}
