//lint:file-ignore U1000 Fields are not unused
//go:generate go run ../cmd/codegen/main.go s2c fmt math
//go:generate go fmt

package GameService

import GwPacket "gw1/server/gwpacket"

type NestedUint32 []uint32
type VarUint32 []uint32

// opcode: 0x009a
type AgentUpdateNPCName struct {
	agentId int //wire:uint32
	encName VarUTF16
}

// opcode: 0x000c
type ServerPingRequest struct {
	unk1 int //wire:uint16
	unk2 int //wire:uint32
}

// opcode: 0x001e
type AgentMovementTick struct {
	delta int //wire:uint32
}

// opcode: 0x0029
type MoveToPointS2C struct {
	agentId int //wire:uint32
	x       float32
	y       float32
	plane   int //wire:uint16
	unk1    int //wire:uint16,val:0
}

// opcode: 0x001b
type PvPItemsEnd struct {
}

/*
	0x0187: {
		0x1005,
		0x1417,
		0x0204,
		0x4000b,
		0x0006,
	},
*/
// opcode: 0x187
type CharCreationFinish struct {
	charUuid []byte //len:16
	name     string
	mapId    int    //wire:uint16
	unk3     []byte //wire:VarByte
}

// opcode: 0x0188
type CharCreationStart struct {
}

// opcode: 0x018a
type CharCreationError struct {
	errorCode int //wire:uint32
}

// opcode: 0x0098
type UpdateCurrentMapId struct {
	mapId int //wire:uint16
	unk1  int //wire:uint8,val:0
}

// opcode: 0x002c
type AgentUpdatePosition struct {
	agentId int // wire:uint32
	x       float32
	y       float32
	plane   int // wire:uint16
}

// opcode: 0x006d
type AgentUpdateVisualEquipment struct {
	agentId int //wire:uint32
	unk1    int //wire:uint32,val:0
	unk2    int //wire:uint32,val:0
	unk3    int //wire:uint32,val:0
	unk4    int //wire:uint32,val:0
	unk5    int //wire:uint32,val:0
	unk6    int //wire:uint32,val:0
	unk7    int //wire:uint32,val:0
	unk8    int //wire:uint32,val:0
	unk9    int //wire:uint32,val:0
}

// opcode: 0x0055
type AgentUpdateNPCProperties struct {
	agentId           int //wire:uint32
	fileId            int //wire:uint32
	unk1              int //wire:uint32,val:0
	scale             int //wire:uint32,val:0x64000000
	unk2              int //wire:uint32,val:0
	flags             int //wire:uint32,val:0x20C
	primaryProfession int //wire:uint8
	level             int //wire:uint8
	unk3              VarUTF16
}

// opcode: 0x0056
type AgentUpdateNPCModel struct {
	npcId   int //wire:uint32
	unk1    int //wire:uint16,val:1
	modelId int //wire:uint32
}

// opcode: 0x0020
type AgentSpawned struct {
	agentId         int //wire:uint32
	agentType       int //wire:uint32
	unk1            int // wire:uint8
	unk2            int // wire:uint8
	posX            float32
	posY            float32
	plane           int //wire:uint16
	facingX         float32
	facingY         float32
	unk3            int //wire:uint8,val:1
	speed           float32
	unk4            float32 //val:1.0
	unk5            int     //wire:uint32,val:0x41400000
	allegianceFlags int     //wire:uint32
	unk6            int     //wire:uint32,val:0
	unk7            int     //wire:uint32,val:0
	unk8            int     //wire:uint32,val:0
	unk9            int     //wire:uint32,val:0
	unk10           int     //wire:uint32,val:0
	unk11           float32 //val:0.0
	unk12           float32 //val:0.0
	inf1            float32 //val:float32(math.Inf(1))
	inf2            float32 //val:float32(math.Inf(1))
	unk13           int     //wire:uint16,val:0
	unk14           int     //wire:uint32,val:0
	inf3            float32 //val:float32(math.Inf(1))
	inf4            float32 //val:float32(math.Inf(1))
	unk15           int     //wire:uint16,val:0
}

// opcode: 0x0021
type AgentDespawned struct {
	agentId int // wire:uint32
}

// opcode: 0x0194
type InstanceLoadSpawnPoint struct {
	mapFileId   int //wire:uint32
	posX        float32
	posY        float32
	plane       int //wire:uint16
	unk1        int //wire:uint8,val:58
	isCinematic bool
	unk2        []byte //len:8
}

// opcode: 0x0196
type InstanceManifestDone struct {
	unk1 int //wire:uint8
	unk2 int //wire:uint16
	unk3 int //wire:uint32
}

// opcode: 0x0195
type InstanceManifestData struct {
	data VarByte
}

// opcode: 0x0197
type InstanceManifestPhase struct {
	phase int //wire:uint8
}

// opcode: 0x01aa
type ReadyForMapSpawn struct {
	unk1 int //wire:uint32,val:808531509
}

// opcode: 0x0030
type HeroInfo struct {
	unk1 int //wire:uint16,val:0
	unk2 int //wire:uint32,val:0
	unk3 int //wire:uint32,val:0
	unk4 int //wire:uint32,val:1000
	unk5 int //wire:uint32,val:0
	unk6 int //wire:uint32,val:0
	unk7 int //wire:uint32,val:0
}

// opcode: 0x0198
type InstanceLoadInfo struct {
	playerId     int //wire:uint32
	mapId        int //wire:uint16
	isExplorable bool
	district     int //wire:uint32
	languageCode int //wire:uint8
	isObserver   bool
}

// opcode: 0x017c
type InstanceLoadPlayerName struct {
	name string
}

// opcode: 0x0159
type ItemSetProfession struct {
	unk1       int //wire:uint32
	profession int //wire:uint8
}

// opcode: 0x0037
type AgentUpdateAttributePoints struct {
	agentId int //wire:uint32
	points1 int //wire:uint8
	points2 int //wire:uint8
}

// opcode: 0x0185
type InstancePlayerDataStart struct {
}

// opcode: 0x0189
type InstancePlayerDataDone struct {
}

// opcode: 0x00da
type SkillsUnlocked struct {
	unk1 int //wire:uint16,val:0
}

// opcode: 0x008a
type CartographyDataStart struct {
	unk1 int //wire:uint32,val:64
	unk2 int //wire:uint32,val:1280
	unk3 int //wire:uint32,val:73
}

// opcode: 0x0089
type CartographyData struct {
	data []byte
}

// opcode: 0x0093
type MapsUnlocked struct {
	data []byte
}

// opcode: 0x004a
type QuestsInfo struct {
	data VarByte
}

// opcode: 0x00f1
type InstanceLoaded struct {
	unk1 int //wire:uint32,val:1886151033
}

// opcode: 0x00f9
type VanquishProgress struct {
	progress int //wire:uint16
}

// opcode: 0x0058
type AgentCreatePlayer struct {
	playerId       int //wire:uint32
	agentId        int //wire:uint32
	appearanceBits int //wire:uint32
	unk3           int //wire:uint8,val:0
	unk4           int //wire:uint32,val:0
	unk5           int //wire:uint32:val:3435973836
	name           string
}

// opcode: 0x00ef
type AgentInitialEffects struct {
	agentId int //wire:uint32
	effects int //wire:uint32
}

// opcode: 0x0047
type AgentDisplayCape struct {
	agentId int //wire:uint32
	isShown bool
}

// opcode: 0x0022
type AgentSetPlayer struct {
	agentId int //wire:uint32
	unk1    int //wire:uint32,val:3
}

// opcode: 0x006a
type PostProcess struct {
	unk1 int //wire:uint8,val:0
	unk2 int //wire:uint32,val:0
}

// opcode: 0x00a5
type AgentUpdateProfession struct {
	agentId             int //wire:uint32
	primaryProfession   int //wire:uint8
	secondaryProfession int //wire:uint8
}

// opcode: 0x01d2
type PartyMemberStreamEnd struct {
	partyId int //wire:uint16
}

// opcode: 0x00af
type UpdatePartySize struct {
	playerId int //wire:uint16
	unk2     int //wire:uint8
}

// opcode: 0x01d1
type PartyCreate struct {
	partyId int //wire:uint16
}

// opcode: 0x1ca
type PartyPlayerAdd struct {
	partyId        int //wire:uint16
	playerId       int //wire:uint16
	isClientLoaded int //wire:uint8,val:1
}

// opcode: 0x018d
type InstanceLoadFinish struct {
}

// opcode: 0x01bd
type PartySetDifficulty struct {
	isHardMode bool
}

// opcode: 0x01dd
type PartySearchSeek struct {
	unk1 int //wire:uint16,val:0
}

// opcode: 0x0143
type ItemStreamCreate struct {
	itemStreamId int //wire:uint16
	unk1         int //wire:uint8,val:0
}

// opcode: 0x013d
type ItemMovedToLocation struct {
	itemStreamId int //wire:uint16
	itemLocalId  int //wire:uint32
	bagId        int //wire:uint16
	slot         int //wire:uint8
}

// opcode: 0x0147
type ActivateWeaponSet struct {
	weaponSetId int //wire:uint16
	unk1        int //wire:uint8,val:0
}

// opcode: 0x0146
type ItemWeaponSet struct {
	unk1        int //wire:uint16,val:1
	weaponSetId int //wire:uint8
	unk2        int //wire:uint32,val:0
	unk3        int //wire:uint32,val:0
}

// opcode: 0x013e
type InventoryCreateBag struct {
	itemStreamId     int //wire:uint16
	bagType          int //wire:uint8
	bagModelId       int //wire:uint8
	bagId            int //wire:uint16
	capacity         int //wire:uint8
	associatedItemId int //wire:uint32
}

// opcode: 0x0160
type ItemGeneralInfo struct {
	itemLocalId   int //wire:uint32
	fileId        int //wire:uint32
	itemType      int //wire:uint8
	unk1          int //wire:uint8
	dyeColor      int //wire:uint16
	materials     int //wire:uint16
	unk2          int //wire:uint8
	itemFlags     int //wire:uint32
	merchantPrice int //wire:uint32
	itemId        int //wire:uint32
	quantity      int //wire:uint32
	encName       VarUTF16
	modifiers     NestedUint32
}

// opcode: 0x0139
type ItemUpdateName struct {
	itemId int //wire:uint32
	name   string
}

// opcode: 0x00b5
type PlayerUnlockedProfessions struct {
	agentId  int //wire:uint32
	unlocked int //wire:uint32
}

// opcode: 0x00b0
type Unknown00b0 struct {
	playerId1 int //wire:uint16
	playerId2 int //wire:uint16
}

// opcode: 0x00d9
type SkillbarUpdate struct {
	agentId int       //wire:uint32
	unk1    VarUint32 //len:8
	unk2    VarUint32 //len:8
	unk3    int       //wire:uint8,val:1
}

// opcode: 0x009e
type AgentAttrUpdateInt struct {
	attributeId int //wire:uint32
	agentId     int //wire:uint32
	val         int //wire:uint32
}

// opcode: 0x00a1
type AgentAttrUpdateFloat struct {
	attributeId int //wire:uint32
	agentId     int //wire:uint32
	val         float32
}

// opcode: 0x00e8
type PlayerAttrSet struct {
	xp    int //wire:uint32
	unk2  int //wire:uint32,val:0
	unk3  int //wire:uint32,val:0
	unk4  int //wire:uint32,val:0
	unk5  int //wire:uint32,val:0
	unk6  int //wire:uint32,val:0
	unk7  int //wire:uint32,val:0
	unk8  int //wire:uint32,val:0
	unk9  int //wire:uint32,val:0
	level int //wire:uint32
	unk11 int //wire:uint32,val:100
	unk12 int //wire:uint32,val:0
	unk13 int //wire:uint32,val:0
	unk14 int //wire:uint32,val:0
	unk15 int //wire:uint32,val:0
}

// opcode: 0x00b6
type PlayerUpdateProfession struct {
	agentId               int //wire:uint32
	primaryProfessionId   int //wire:uint8
	secondaryProfessionId int //wire:uint8
	unk1                  int //wire:uint8,val:0
}

// opcode: 0x017b
type InstanceLoadHead struct {
	campaign int //wire:uint8,val:2
	unk1     int //wire:uint8,val:0
	unk2     int //wire:uint8,val:0
	unk3     int //wire:uint8,val:0
}

// opcode: 0x005d
type ChatMessageServer struct {
	unk1    int //wire:uint16,val:0
	channel int //wire:uint8
}

// opcode: 0x0060
type ChatMessageLocal struct {
	agentId int //wire:uint16
	channel int // wire:uint8
}

func MarshalChatMessageCore(message string) GwPacket.Out {
	resp := GwPacket.NewOut(0x5c)
	resp.Uint16(len(message) + 3)
	resp.Uint16(0x0108)
	resp.Uint16(0x0107)
	resp.UTF16(message)
	resp.Uint16(0x0001)
	return resp
}

func MarshalChatMessageFromServer(message string, channel int) GwPacket.Out {
	resp := MarshalChatMessageCore(message)
	resp.Merge(MarshalChatMessageServer(channel))
	return resp
}

// opcode: 0x009b
type UpdateDeathPenalty struct {
	agentId           int //wire:uint32
	deathPenaltyBasis int //wire:uint32
}

// opcode: 0x0018
type SetUnlockedHeroes struct {
	unk []uint16
}

// opcode: 0x0033
type MessageOfTheDay struct {
	// note this isn't a regular message. send a regular message 'test123' and you get:
	// Assertion: (codedString[0] & ~WORD_BIT_MORE) >= WORD_VALUE_BASE
	// P:\Code\Engine\Text\TextApi.cpp(585)
	motd string
}
