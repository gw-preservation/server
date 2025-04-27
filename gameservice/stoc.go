package GameService

import (
	GwPacket "gw1/server/gwpacket"
	"math"
)

const (
	opcodePingRequest                = 0x000c
	opcodeInstanceLoaded             = 0x00f1
	opcodeInstanceLoadHead           = 0x017b
	opcodeInstanceLoadFinish         = 0x018d
	opcodeItemStreamCreate           = 0x0143
	opcodeItemSetActiveWeaponSet     = 0x0147
	opcodeItemGeneralInfo            = 0x0160
	opcodeInventoryCreateBag         = 0x013e
	opcodeItemMovedToLocation        = 0x013d
	opcodeItemWeaponSet              = 0x0146
	opcodePlayerUnlockedProfessions  = 0x00b5
	opcodePlayerUpdateProfession     = 0x00b6
	opcodeSkillbarUpdate             = 0x00d9
	opcodeAgentAttrUpdateInt         = 0x009e
	opcodeAgentAttrUpdateFloat       = 0x00a1
	opcodePlayerAttrSet              = 0x00e8
	opcodePartySearchSeek            = 0x01dd
	opcodePartySetDifficulty         = 0x01bd
	opcodePartyMemberStreamEnd       = 0x01d2
	opcodePartyPlayerAdd             = 0x01ca
	opcodePartyCreate                = 0x01d1
	opcodeUpdateAgentPartySize       = 0x00af
	opcodePostProcess                = 0x006a
	opcodeAgentSetPlayer             = 0x0022
	opcodeAgentDisplayCape           = 0x0047
	opcodeAgentInitialEffects        = 0x00ef
	opcodeAgentUpdateProfession      = 0x00a5
	opcodeAgentCreatePlayer          = 0x0058
	opcodeQuestInfo                  = 0x004a
	opcodeAccountCurrencyUpdate      = 0x000f
	opcodeMapsUnlocked               = 0x0093
	opcodeCartographyData            = 0x0089
	opcodeSkillsUnlocked             = 0x00da
	opcodeChatMessageCore            = 0x005c
	opcodeChatMessageServer          = 0x005d
	opcodeChatMessageNPC             = 0x005e
	opcodeChatMessageGlobal          = 0x005f
	opcodeChatMessagelocal           = 0x0060
	opcodeChatMessagePrivate         = 0x000e
	opcodeItemSetProfession          = 0x0159
	opcodeInstancePlayerDataStart    = 0x0185
	opcodeInstancePlayerDataDone     = 0x0189
	opcodeAgentUpdateAttributePoints = 0x0037
	opcodeInstanceLoadPlayerName     = 0x017c
	opcodeInstanceLoadInfo           = 0x0198
	opcodeHeroInfo                   = 0x0030
	opcodeUpdateMapId                = 0x0098
	opcodeReadyForMapSpawn           = 0x01aa
	opcodeInstanceManifestPhase      = 0x0197
	opcodeInstanceManifestData       = 0x0195
	opcodeInstanceManifestDone       = 0x0196
	opcodeInstanceLoadSpawnPoint     = 0x0194
	opcodeAgentSpawned               = 0x0020
	opcodeAgentUpdateVisualEquipment = 0x006d
	opcodeUpdateCurrentMap           = 0x0098
	opcodeCharCreationError          = 0x018a
	opcodeCharCreationStart          = 0x0188
	opcodePvpItemEnd                 = 0x001b // unsure about this one
	opcodeMoveToPoint                = 0x0029
	opcodeAgentMovementTick          = 0x001e
)

func newPingRequest(unk1, unk2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePingRequest)
	resp.Uint16(unk1)
	resp.Uint32(unk2)
	return
}

func newAgentMovementTick(delta int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentMovementTick)
	resp.Uint32(delta)
	return
}

func newMoveToPoint(agentId int, x, y float32) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeMoveToPoint)
	resp.Uint32(agentId)
	resp.Float32(x)
	resp.Float32(y)
	resp.Uint16(0)
	resp.Uint16(0)
	return
}

func newPvpItemEnd() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePvpItemEnd)
	return
}
func newCharCreationStart() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeCharCreationStart)
	return
}
func newCharCreationError(errorCode int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeCharCreationError)
	resp.Uint32(errorCode)
	return
}

func newUpdateCurrentMap(unk1, unk2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeUpdateCurrentMap)
	resp.Uint16(514)
	resp.Uint8(0)
	return
}

func newAgentUpdateVisualEquipment(agentId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentUpdateVisualEquipment)
	resp.Uint32(agentId)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(10)
	resp.Uint32(13)
	resp.Uint32(11)
	resp.Uint32(14)
	resp.Uint32(12)
	resp.Uint32(0)
	resp.Uint32(0)
	return
}

func newAgentSpawned(agentId, modelId, agentType int, positionX, positionY float32, plane int, facingX, facingY float32, speed float32) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentSpawned)
	resp.Uint32(agentId)
	resp.Uint32(modelId)
	resp.Uint8(agentType)
	resp.Uint8(5)
	resp.Float32(positionX)
	resp.Float32(positionY)
	resp.Uint16(plane)
	resp.Float32(facingX)
	resp.Float32(facingY)
	resp.Uint8(1)
	resp.Float32(speed)
	resp.Float32(1.0)
	resp.Uint32(0x41400000)
	resp.Uint32(0x706c6179) // modelType
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Float32(0)
	resp.Float32(0)
	resp.Float32(float32(math.Inf(1)))
	resp.Float32(float32(math.Inf(1)))
	resp.Uint16(0)
	resp.Uint32(0)
	resp.Float32(float32(math.Inf(1)))
	resp.Float32(float32(math.Inf(1)))
	resp.Uint16(0)

	return

}

func newInstanceLoadSpawnPoint(mapFileId int, posX, posY float32, plane int, isCinematic bool) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceLoadSpawnPoint)
	resp.Uint32(mapFileId) // MapFile
	resp.Float32(posX)     // PosX
	resp.Float32(posY)     // PosY
	resp.Uint16(plane)     // Plane
	resp.Uint8(58)         // unk
	if isCinematic {
		resp.Uint8(1)
	} else {
		resp.Uint8(0) // isCinematic
	}
	// FortRanik =   03 3B 48 45 02 AA DB 01
	// FoiblesFair = CD 49 03 CC 17 A7 DB 01
	resp.Bytes([]byte{0xcd, 0x49, 0x03, 0xcc, 0x17, 0xa7, 0xdb, 0x01}) // unk
	//resp.Bytes([]byte{0x03, 0x3b, 0x48, 0x45, 0x02, 0xaa, 0xdb, 0x01})
	return
}

func newInstanceManifestDone(unk1, unk2, unk3 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceManifestDone)
	resp.Uint8(unk1)
	resp.Uint16(unk2)
	resp.Uint32(unk3)
	return
}

func newInstanceManifestData(data []byte) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceManifestData)
	resp.Uint16(len(data))
	resp.Bytes(data)
	return
}

func newInstanceManifestPhase(phase int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceManifestPhase)
	resp.Uint8(phase)
	return
}

func newReadyForMapSpawn() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeReadyForMapSpawn)
	resp.Uint32(808531509)
	return
}

func newHeroInfo() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeHeroInfo)
	resp.Uint16(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(1000)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	return
}

func newUpdateMapId(mapId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeUpdateMapId)
	resp.Uint16(mapId) // MapID?
	resp.Uint8(0)
	return
}

func newInstanceLoadInfo(agentId, mapId int, explorable bool, district, language int, observer bool) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceLoadInfo)
	resp.Uint32(agentId)
	resp.Uint16(mapId)
	if explorable {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}
	resp.Uint32(district)
	resp.Uint8(language)
	if observer {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}
	return
}

func newInstanceLoadPlayerName(name string) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceLoadPlayerName)
	resp.UTF16WithLengthPrefix(name)
	return
}

func newItemSetProfession(unk1, unk2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeItemSetProfession)
	resp.Uint32(unk1)
	resp.Uint8(unk2)
	return
}

func newAgentUpdateAttributePoints(agentId, points1, points2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentUpdateAttributePoints)
	resp.Uint32(agentId) // AgentID
	resp.Uint8(points1)
	resp.Uint8(points2)
	return
}

func newInstancePlayerDataStart() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstancePlayerDataStart)
	return resp
}
func newInstancePlayerDataDone() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstancePlayerDataDone)
	return resp
}

func newSkillsUnlocked() (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeSkillsUnlocked)
	resp.Uint16(0)
	return
}

func newCartographyData(cartographyData []byte) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeCartographyData)
	resp.Bytes(cartographyData)
	return
}

func newMapsUnlocked(mapData []byte) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeMapsUnlocked)
	resp.Bytes(mapData)
	return
}

func newAccountCurrencyUpdate(unk1, unk2, unk3 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAccountCurrencyUpdate)
	resp.Uint16(unk1)
	resp.Uint16(unk2)
	resp.Uint16(unk3)
	return
}

func newQuestInfo(questBytes []byte) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeQuestInfo)
	resp.Uint16(len(questBytes)) // this would change for active quests
	if len(questBytes) > 0 {
		resp.Bytes(questBytes)
	}
	return
}

func newInstanceLoaded(unk1 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInstanceLoaded)
	resp.Uint32(unk1)
	return
}

func newVanquishProgress(progress int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(0x00f9)
	resp.Uint16(progress)
	return
}

func newAgentCreatePlayer(unk1, agentId, unk2, unk3, unk4, unk5 int, name string) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentCreatePlayer)
	resp.Uint32(unk1)
	resp.Uint32(agentId) // AgentID
	resp.Uint32(unk2)
	resp.Uint8(unk3)
	resp.Uint32(unk4)
	resp.Uint32(unk5)
	resp.UTF16WithLengthPrefix(name) // me
	return
}

func newAgentUpdateProfession(agentId int, primary int, secondary int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentUpdateProfession)
	resp.Uint32(agentId)
	resp.Uint8(primary)
	resp.Uint8(secondary)
	return
}

func newAgentInitialEffects(agentId int, effects uint32) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentInitialEffects)
	resp.Uint32(agentId)
	resp.Uint32(int(effects))
	return
}

func newAgentDisplayCape(agentId int, displayed bool) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentDisplayCape)
	resp.Uint32(agentId)
	if displayed {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}
	return
}

func newAgentSetPlayer(agentId, unk2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeAgentSetPlayer)
	resp.Uint32(agentId)
	resp.Uint32(3)
	return
}

func newPostProcess(unk1, unk2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePostProcess)
	resp.Uint8(unk1)
	resp.Uint32(unk2)
	return
}

func newPartyMemberStreamEnd(partyStreamId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePartyMemberStreamEnd)
	resp.Uint16(partyStreamId)
	return
}

func newUpdateAgentPartySize(unk1 int, unk2 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeUpdateAgentPartySize)
	resp.Uint16(unk1)
	resp.Uint8(unk2)
	return
}

func newPartyCreate(partyStreamId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePartyCreate)
	resp.Uint16(partyStreamId)
	return
}

func newPartyPlayerAdd(partyStreamId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePartyPlayerAdd)
	resp.Uint16(partyStreamId)
	resp.Uint16(1)
	resp.Uint8(1)
	return
}

func newInstanceLoadFinish() GwPacket.Out {
	resp := GwPacket.NewOut(opcodeInstanceLoadFinish)
	return resp
}

func newPartySetDifficulty(hardMode bool) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePartySetDifficulty)
	if hardMode {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}
	return
}

func newPartySearchSeek(seeking bool) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodePartySearchSeek)
	if seeking {
		resp.Uint16(1)
	} else {
		resp.Uint16(0)
	}
	return
}

func newItemStreamCreate(itemStreamId int) GwPacket.Out {
	resp := GwPacket.NewOut(opcodeItemStreamCreate)
	resp.Uint16(itemStreamId)
	resp.Uint8(0)
	return resp
}

func newItemMovedTolocation(itemStreamId, itemLocalId, pageId, slot int) GwPacket.Out {
	resp := GwPacket.NewOut(opcodeItemMovedToLocation)
	resp.Uint16(itemStreamId)
	resp.Uint32(itemLocalId)
	resp.Uint16(pageId)
	resp.Uint8(slot)
	return resp
}

func newActivateWeaponSet(weaponSetId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeItemSetActiveWeaponSet)
	resp.Uint16(weaponSetId)
	resp.Uint8(0)
	return
}

func newItemWeaponSet(weaponSetId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeItemWeaponSet)
	resp.Uint16(1)
	resp.Uint8(weaponSetId)
	resp.Uint32(0)
	resp.Uint32(0)
	return
}

func newInventoryCreateBag(itemStreamId, bagId int, unk1 int, unk2 int, capacity int, unk3 int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeInventoryCreateBag)
	resp.Uint16(itemStreamId)
	resp.Uint8(bagId)
	resp.Uint8(unk1)
	resp.Uint16(unk2)
	resp.Uint8(capacity)
	resp.Uint32(unk3)
	return resp
}

type itemGeneralInfo struct {
	itemLocalId   int
	fileId        int
	itemType      int
	unk1          int
	dyeColor      int
	materials     int
	unk2          int
	itemFlags     int
	merchantPrice int
	itemId        int
	quantity      int
	encNameBytes  []byte
	unk3          int
}

func newItemGeneralInfo(info itemGeneralInfo) GwPacket.Out {
	resp := GwPacket.NewOut(opcodeItemGeneralInfo)
	resp.Uint32(info.itemLocalId)
	resp.Uint32(info.fileId)
	resp.Uint8(info.itemType)
	resp.Uint8(info.unk1)
	resp.Uint16(info.dyeColor)
	resp.Uint16(info.materials)
	resp.Uint8(info.unk2)
	resp.Uint32(info.itemFlags)
	resp.Uint32(info.merchantPrice)
	resp.Uint32(info.itemId)
	resp.Uint32(info.quantity)
	resp.Uint16(len(info.encNameBytes) / 2)
	resp.Bytes(info.encNameBytes)
	resp.Uint8(1)
	resp.Uint32(info.unk3)
	return resp
}

func newPlayerUnlockedProfessions(argentId int) GwPacket.Out {
	resp := GwPacket.NewOut(opcodePlayerUnlockedProfessions)
	resp.Uint32(argentId)
	resp.Uint32(0) // No professions unlocked
	return resp
}

func newSkillbarUpdate(agentId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(opcodeSkillbarUpdate)
	resp.Uint32(agentId)
	resp.Uint16(8) // Array of 8
	for range 8 {
		resp.Uint32(0)
	}
	resp.Uint16(8) // Array of 8
	for range 8 {
		resp.Uint32(0)
	}
	resp.Uint8(1)
	return
}

func newAgentAttrUpdateInt(agentId, attributeId, value int) GwPacket.Out {
	resp := GwPacket.NewOut(opcodeAgentAttrUpdateInt)
	resp.Uint32(attributeId) // attribute id
	resp.Uint32(agentId)     // AgentID
	resp.Uint32(value)       // value
	return resp
}

func newAgentAttrUpdateFloat(agentId, attributeId, value int) GwPacket.Out {
	resp := GwPacket.NewOut(opcodeAgentAttrUpdateFloat)
	resp.Uint32(attributeId) // attribute id
	resp.Uint32(agentId)     // AgentID
	resp.Uint32(value)       // value
	return resp
}

func newPlayerAttrSet() GwPacket.Out {
	resp := GwPacket.NewOut(opcodePlayerAttrSet)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(9000)
	resp.Uint32(9000)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(1)
	resp.Uint32(100)
	resp.Uint32(0)
	resp.Uint32(63000)
	resp.Uint32(0)
	resp.Uint32(0)
	return resp
}

func newPlayerUpdateProfession(agentId, primaryProfessionId int, secondaryProfessionId int) GwPacket.Out {
	resp := GwPacket.NewOut(opcodePlayerUpdateProfession)
	resp.Uint32(agentId) // AgentID
	resp.Uint8(primaryProfessionId)
	resp.Uint8(secondaryProfessionId)
	resp.Uint8(0)
	return resp
}

func newInstanceLoadHead() GwPacket.Out {
	resp := GwPacket.NewOut(opcodeInstanceLoadHead)
	// the next uint8 affects what campaigns are enabled
	resp.Uint8(0x02) //0x2 = Prophecies only
	resp.Uint8(0x00)
	resp.Uint8(0)
	resp.Uint8(0)
	return resp
}

func newChatMessageOwner(ownerId int, color int) GwPacket.Out {
	resp := GwPacket.NewOut(0x0060)
	resp.Uint16(ownerId)
	resp.Uint8(color)
	return resp
}

func newChatMessageServer(color int) GwPacket.Out {
	resp := GwPacket.NewOut(0x005d)
	resp.Uint16(0)
	resp.Uint8(color)
	return resp
}

func newChatMessageFromServer(message string, color int) GwPacket.Out {
	// color=7 makes text in red in middle of screen -- used for ie quick skill alerts
	resp := GwPacket.NewOut(opcodeChatMessageCore)
	resp.Uint16(len(message) + 3)
	resp.Uint16(0x0108)
	resp.Uint16(0x0107)
	resp.UTF16(message)
	resp.Uint8(0x01)
	resp.Uint8(0x00)
	resp.Merge(newChatMessageServer(color))
	return resp
}
