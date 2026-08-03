//lint:file-ignore U1000 Fields are not unused
//go:generate go run ../cmd/codegen/main.go c2s errors fmt
//go:generate go fmt

package gameservice

type VarByte []byte
type VarUTF16 []byte

// opcode: 0x808a
type CreateCharacterFinish struct {
	name       string //maxlen:20
	appearance int    // wire:uint32
	ignore     int    // wire:uint32
}

// opcode: 0x808f
type InstanceLoadRequestPlayers struct {
	unk1 []byte // len:16
}

// opcode: 0x8091
type Unknown8091 struct {
	unk1 VarByte
}

// opcode: 0x0500
type VerifyClientConnection struct {
	clientVersion int    //wire:uint32
	unk3          int    //wire:uint16
	unk4          int    //wire:uint32
	instanceTag   int    //wire:uint32
	mapId         int    //wire:uint32
	securityTag   int    //wire:uint32
	accountUUID   []byte //len:16
	characterUUID []byte //len:16
	unk5          int    //wire:uint32
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
	unk1         int // wire:uint8
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
	driverName    string //maxlen:64
	driverVersion string //maxlen:20
}

// opcode: 0x4200
type ClientSeed struct {
	seed []byte //len:64
}

// opcode: 0x8063
type ChatMessage struct {
	agentId int    //wire:uint32
	message string //maxlen:138
}

// opcode: 0x803d
type MoveToPoint struct {
	x     float32
	y     float32
	plane int //wire:uint32
}

// opcode: 0x803f
type RotateAgent struct {
	unk1 int // wire:uint32
	unk2 int // wire:uint32
}

// opcode: 0x8046
type LastPosBeforeMoveCancelled struct {
	x    float32
	y    float32
	unk2 int // wire:uint32
}

// opcode: 0x803c
type MovementUpdate struct {
	posX    float32
	posY    float32
	unk1    int // wire:uint32
	facingX float32
	facingY float32
	dir     int // wire:uint32
}

// opcode: 0x80c0
type UpdateTarget struct {
	targetAgentId int // wire:uint32
	agentId2      int // wire:uint32
}

// opcode: 0x8038
type InteractAgent struct {
	agentId int // wire:uint32
	action  int // wire:uint8
}

// opcode: 0x8027
type CancelInteraction struct {
}

// opcode: 0x800c
type ClientPingRequest struct {
}

// opcode: 0x8087
type InstanceLoadRequestSpawnPoint struct {
}

// opcode: 0x8088
type CreateCharRequestPlayer struct {
}

// opcode: 0x8089
type CreateCharRequestItems struct {
}

// opcode: 0x8008
type ClientDisconnect struct {
}

// opcode: 0x8090
type Unknown8090 struct {
	unk1 int //wire:uint8
	unk2 int //wire:uint8
}

// opcode: 0x80b0
type MapTravelToOutpost struct {
	mapId    int //wire:uint16
	region   int //wire:uint8
	district int //wire:uint16
	language int //wire:uint8
	unk1     int //wire:uint8
}

// opcode: 0x802f
type EquipItem struct {
	itemLocalId int // wire:uint32
}

// opcode: 0x8068
type DestroyItem struct {
	itemLocalId int // wire:uint32
}

// opcode: 0x80a0
type PartyInvite struct {
	name string //maxlen:20
}

// opcode: 0x8090
type InstanceLoadRequestItems struct {
	unk1 int // wire:uint8
	unk2 int // wire:uint8
}
