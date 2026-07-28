package GameService

import (
	"bytes"
	"fmt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	Item "gw1/server/item"
	"math/rand"
	"strings"

	"github.com/rs/zerolog"
)

const equipmentBagIndex = 1

type Player struct {
	Agent
	playerId           int
	conn               *GSConn
	questBytes         []byte
	connectedInstance  *Instance
	log                zerolog.Logger
	readyForAgentTicks bool // TODO: refactor into LoadingState / ReadyState
	xp                 int
	itemMgr            ItemMgr

	charCreationDyes [7]int

	isTransfer bool
	dbAcc      db.Account
	dbChar     db.Character
}

func NewPlayer(conn *GSConn, logCtx zerolog.Logger) *Player {
	p := &Player{
		conn:               conn,
		questBytes:         make([]byte, 0),
		readyForAgentTicks: false,
		isTransfer:         false,
	}
	p.itemMgr = NewItemMgr(p)
	p.allegianceFlags = 0x706c6179
	p.uuid = rand.Uint64()
	p.log = logCtx.With().Uint64("uuid", p.uuid).Logger()
	p.isPlayer = true
	return p
}

func (p *Player) syncFromDB(char db.Character) {
	p.dbChar = char
	p.name = char.Name
	p.primaryProfession = int(char.ProfessionPrimary)
	p.secondaryProfession = int(char.ProfessionSecondary)
	p.level = int(char.Level)
	p.xp = int(char.XP)
}

func (p *Player) TransmitItems() {
	dbBags, ok := db.GetBagsForCharacterByID(p.dbChar.ID)
	if !ok {
		p.log.Error().Uint64("id", p.dbChar.ID).Msg("failed to get bags for character")
		return
	}
	p.itemMgr.Load(dbBags, p.dbChar.ID)
}

func (p *Player) TryEquipItem(itemLocalId int) {
	itm, ok := p.itemMgr.GetItemByLocalId(itemLocalId)
	if !ok {
		p.log.Warn().Int("itemLocalId", itemLocalId).Msg("Player attempted to equip an item that does not exist")
		return
	}
	p.log.Info().Str("name", itm.Name()).Msg("Try Equip")
	equipSlot := itm.GetEquipSlot()
	equipVisualSlot := itm.GetVisualEquipSlot()
	if equipSlot == Item.EquipSlotUnknown || equipVisualSlot == Item.EquipVisualSlotUnknown {
		p.log.Warn().Int("itemLocalId", itemLocalId).Msg("Player attempted to equip an item that cannot be equipped")
		return
	}
	err := p.itemMgr.MoveItemByLocalId(itemLocalId, 1, int(equipSlot))
	if err != nil {
		p.log.Warn().Int("itemLocalId", itemLocalId).Err(err).Msg("Unable to remove item by local id")
		return
	}
	p.EnqueuePacket(MarshalAgentUpdateVisualEquipment2(p.agentId, int(equipVisualSlot), itemLocalId))
	if err := p.itemMgr.SyncToDB(); err != nil {
		p.log.Error().Err(err).Msg("failed to sync items after equip")
	}
}

func (p *Player) EnqueuePacket(out GwPacket.Out) {
	p.conn.EnqueuePacket(out)
}

func (p *Player) Disconnect() {
	p.conn.Close()
}

func (p *Player) SendChat(msg string, color int) {
	p.conn.EnqueuePacket(MarshalChatMessageFromServer(fmt.Sprintf("(server) %s", msg), color))
}

func (p *Player) SendChatWarning(msg string) {
	p.SendChat(msg, 13)
}

func (p *Player) SendChatInfo(msg string) {
	p.SendChat(msg, 3)
}

func (p *Player) SendChatDebug(msg string) {
	p.SendChat(msg, 4)
}

func (p *Player) SendChatColorTest() {
	for i := range 14 {
		p.SendChat(fmt.Sprintf("index: %d", i), i)
	}
}

func (p *Player) SendWelcomeChatMessage() {
	if !p.isTransfer {
		p.EnqueuePacket(MarshalChatMessageFromServer("Welcome to the server!", 5))
		p.EnqueuePacket(MarshalChatMessageFromServer(fmt.Sprintf("Players online: %d", InstanceManager.NumPlayersOnline()), 5))
	}
}

func (p *Player) OnC2SVerifyConnection(payload VerifyClientConnection) {
	// We should validate now, to check the request is valid
	// First check token:
	info, ok := ValidateConnectionToken(uint32(payload.securityTag))
	if !ok {
		p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("invalid securityTag")
		p.Disconnect()
		return
	}

	inst, err := InstanceManager.GetOrCreateInstanceByMapId(payload.mapId)
	if inst == nil || err != nil {
		p.log.Error().Err(err).Msg("unable to create instance")
		p.Disconnect()
		return
	}

	if payload.instanceTag != int(info.InstanceTag) {
		p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("instanceTag does not match expected value")
		p.Disconnect()
		return
	}

	if info.InstanceTag != inst.GetTag() {
		p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("instance no longer has instanceTag specified by token")
		p.Disconnect()
		return
	}
	p.isTransfer = info.IsTransfer
	verified := false
	acc, ok := db.GetFullAccountByUUID(payload.accountUUID[:])
	if !ok {
		p.log.Error().Str("accUUID", db.UUIDStr(payload.accountUUID[:])).Msg("no such account")
		p.Disconnect()
		return
	}
	p.dbAcc = acc
	if payload.mapId == 0 {
		p.log.Debug().Msg("Skip UUID check - entering CharCreation instance")
	} else {
		// Check character UUID exists:
		for _, char := range acc.Characters {
			if bytes.Equal(char.UUID, payload.characterUUID[:]) {
				p.syncFromDB(char)
				verified = true
				break
			}
		}
		if !verified {
			p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("no such character")
			p.Disconnect()
			return
		}
	}

	// TODO: Here we should verify the map is adjacent to the LastOutpostID if its explorable!

	// Hook client up to an instance
	p.connectedInstance = inst

	p.log.Debug().Str("instanceTag", fmt.Sprintf("%08x", payload.instanceTag)).Str("securityTag", fmt.Sprintf("%08x", payload.securityTag)).Int("mapId", payload.mapId).Int("unk3", payload.unk3).Int("unk4", payload.unk4).Int("unk5", payload.unk5).Int("unk6", payload.unk6).Msg("VerifyClientConnection")
}

func (p *Player) OnUserDisconnected() {
	if err := p.itemMgr.SyncToDB(); err != nil {
		p.log.Error().Err(err).Msg("failed to sync items to database on disconnect")
	}
}

func (p *Player) OnC2SUpdateProfessionChoice(payload UpdateProfessionChoice) {
	p.log.Debug().
		Int("profession", payload.professionId).
		Int("unk1", payload.unk1).
		Msg("UpdateProfessionChoice")
	if payload.professionId == p.primaryProfession {
		return
	}
	// TODO: validate we're actually in char creation instance!
	// TODO: validate profession id
	p.primaryProfession = payload.professionId
	// Marshal 1: delete equipped items
	// Remove items in equipped slots
	numEquipSlots := p.itemMgr.GetNumSlotsInBag(1)
	for slotIndex := range numEquipSlots {
		p.itemMgr.RemoveItemInSlot(1, slotIndex)
	}
	// Marshal 2: new appearance base?
	p.EnqueuePacket(MarshalPlayerUpdateProfession(p.agentId, p.primaryProfession, 0))
	p.EnqueuePacket(MarshalSkillsUnlocked())
	// Marshal 3: create new equipped items

	var equips []Item.ItemId
	switch payload.professionId {
	case 1:
		equips = Item.DefaultEquipmentWarrior
	case 2:
		equips = Item.DefaultEquipmentRanger
	case 3:
		equips = Item.DefaultEquipmentMonk
	case 4:
		equips = Item.DefaultEquipmentNecromancer
	case 5:
		equips = Item.DefaultEquipmentMesmer
	case 6:
		equips = Item.DefaultEquipmentElementalist
	}
	for _, itemid := range equips {
		// Spawn new items in equipped slots
		itm := Item.GetItemDefinitionById(itemid)
		itmlid, err := p.itemMgr.AddItemToSlot(0, 0, itm, itemid, 0)
		if err != nil {
			p.log.Error().Err(err).Msg("unable to add item to slot during profession change")
			return
		}
		err = p.itemMgr.MoveItemByLocalId(itmlid, 1, int(itm.GetEquipSlot()))
		if err != nil {
			p.log.Error().Err(err).Msg("unable to move item to equipment slot")
			return
		}
	}
}

func (p *Player) equipTest(profession string) {
	p.log.Info().Str("profession", profession).Msg("Start equipment test")

	// Remove items in equipped slots
	numEquipSlots := p.itemMgr.GetNumSlotsInBag(1)
	for slotIndex := range numEquipSlots {
		p.itemMgr.RemoveItemInSlot(1, slotIndex)
	}
	var equipmentIds []Item.ItemId
	switch profession {
	case "warrior":
		equipmentIds = Item.DefaultEquipmentWarrior
	case "ranger":
		equipmentIds = Item.DefaultEquipmentRanger
	}
	for _, itemid := range equipmentIds {
		// Spawn new items in equipped slots
		itm := Item.GetItemDefinitionById(itemid)
		itmlid, err := p.itemMgr.AddItemToSlot(1, int(itm.GetEquipSlot()), itm, itemid, 0)
		if err != nil {
			p.log.Error().Err(err).Msg("unable to add item to slot")
			return
		}
		p.EnqueuePacket(MarshalAgentUpdateVisualEquipment2(p.agentId, int(itm.GetVisualEquipSlot()), itmlid))
	}
}

func (p *Player) OnC2SDyeEquipment(payload DyeEquipment) {
	if !p.connectedInstance.IsCharCreationInstance() {
		return
	}
	if payload.color < 0 || payload.color > 9 {
		p.log.Warn().Int("color", payload.color).Msg("invalid dye color")
		return
	}
	if payload.slot < 0 || payload.slot > 6 {
		p.log.Warn().Int("slot", payload.slot).Msg("invalid dye slot")
		return
	}

	lid, err := p.itemMgr.GetLocalIdForSlot(equipmentBagIndex, payload.slot)
	if err != nil {
		p.log.Error().Err(err).Msg("error calling GetLocalIdForSlot")
		return
	}
	if lid == -1 {
		p.log.Debug().Int("slot", payload.slot).Msg("no item in slot to dye")
		return
	}

	item, ok := p.itemMgr.GetItemByLocalId(lid)
	if !ok {
		p.log.Error().Int("lid", lid).Msg("item not found for local id")
		return
	}

	p.applyDyeToItem(lid, item, payload.color)
	p.charCreationDyes[payload.slot] = payload.color
	p.itemMgr.UpdateSlotDye(equipmentBagIndex, payload.slot, uint8(payload.color))
}

func (p *Player) applyDyeToItem(lid int, item Item.Item, color int) {
	p.EnqueuePacket(MarshalItemGeneralInfo(
		lid,
		int(item.ModelFileId()),
		int(item.Type()),
		7, // dye update metadata type
		color,
		0, // materials
		0, // unk2
		item.ComputeInteractionFlags(),
		item.MerchValue(),
		lid,
		1, // quantity
		item.EncodeName(),
		item.MarshalModifiers(),
	))
	p.EnqueuePacket(MarshalItemSetProfession(lid, p.primaryProfession))
	p.EnqueuePacket(MarshalUnknownAfterDyeSuccess(lid, lid))
}

func (p *Player) sendInstanceLoadSpawnPoint() {
	p.log.Debug().Msg("InstanceLoadRequestSpawnPoint")
	inst := *p.connectedInstance
	p.EnqueuePacket(MarshalInstanceLoadSpawnPoint(int(inst.definition.MapFileId), p.posX, p.posY, p.plane, false, []byte{0xcd, 0x49, 0x03, 0xcc, 0x17, 0xa7, 0xdb, 0x01}))
}

func (p *Player) sendInstanceLoadRequestPlayers(payload InstanceLoadRequestPlayers) {
	p.log.Debug().Hex("unkBlob", payload.unk1).Int("playerAgentId", p.agentId).Msg("InstanceLoadRequestPlayers")

	p.sendSkillAndProfessionData()
	p.sendWorldSyncData()
	p.sendPlayerAttributes()
	p.spawnPlayerAgent()
	p.sendPartySetup()

	p.EnqueuePacket(MarshalInstanceLoadFinish())
}

func (p *Player) sendSkillAndProfessionData() {
	p.sendUnlockedSkills()
	p.sendSkillbar()
	p.sendAttributePointsRemaining()
	p.sendProfession()
	p.sendUnlockedProfessions()
	p.EnqueuePacket(GwPacket.NewOut(0x001b))
}

func (p *Player) sendWorldSyncData() {
	p.sendUnlockedPvpHeroes()
	p.sendQuestInfoSync()
	p.sendMapsUnlockedSync()
	p.sendCartographyData()
	p.EnqueuePacket(MarshalInstanceLoaded())
}

func (p *Player) sendPlayerAttributes() {
	p.EnqueuePacket(MarshalAgentAttrUpdateInt(41, p.agentId, 25))      // energy
	p.EnqueuePacket(MarshalAgentAttrUpdateInt(42, p.agentId, 100))     // health
	p.EnqueuePacket(MarshalAgentAttrUpdateInt(36, p.agentId, p.level)) // level
	p.EnqueuePacket(MarshalUpdateDeathPenalty(p.agentId, 100))
	p.EnqueuePacket(MarshalPlayerAttrSet(int(p.xp), p.level))

	// REVERSE THIS MORE:
	resp := GwPacket.NewOut(0x00ee)
	resp.Uint32(255)
	resp.Uint32(255)
	p.EnqueuePacket(resp)

	p.sendAttributeUpdateFloat(43)

	// REVERSE THIS MORE:
	resp = GwPacket.NewOut(0x114)
	resp.Uint32(1)
	resp.Uint32(0)
	p.EnqueuePacket(resp)
}

func (p *Player) spawnPlayerAgent() {
	p.EnqueuePacket(MarshalAgentCreatePlayer(p.playerId, p.agentId, int(p.dbChar.AppearanceBits), p.name))
	p.connectedInstance.SendActiveAgents(p)

	// REVERSE THIS MORE:
	p.EnqueuePacket(MarshalUnknown00b0(p.playerId, p.playerId))

	// GAME_SMSG_AGENT_INITIAL_EFFECTS
	// 0x200 enables GM effect (0010 0000 0000)
	p.EnqueuePacket(MarshalAgentInitialEffects(p.agentId, 0))

	p.EnqueuePacket(MarshalAgentUpdateProfession(p.agentId, p.primaryProfession, p.secondaryProfession))

	// GAME_SMSG_AGENT_SPAWNED - player
	agentType := 0x30000000
	agentType |= p.playerId
	allegianceFlags := 0x706c6179
	plane := 0
	facingX := float32(0)
	facingY := float32(0)
	speed := float32(288) // 1x speed
	p.EnqueuePacket(MarshalAgentSpawned(
		p.agentId,
		agentType,
		1,
		5,
		p.posX,
		p.posY,
		plane,
		facingX,
		facingY,
		speed,
		allegianceFlags,
	))
	p.EnqueuePacket(MarshalAgentSetPlayer(p.agentId))

	p.EnqueuePacket(MarshalAgentUpdateVisualEquipment(p.agentId))
	for i := range p.itemMgr.GetNumSlotsInBag(1) {
		lid, err := p.itemMgr.GetLocalIdForSlot(1, i)
		if err != nil || lid == -1 {
			continue
		}
		item, err := p.itemMgr.GetItemInSlot(1, i)
		if err != nil {
			continue
		}
		p.EnqueuePacket(MarshalAgentUpdateVisualEquipment2(p.agentId, int(item.GetVisualEquipSlot()), lid))
	}

	// GAME_SMSG_AGENT_DISPLAY_CAPE
	p.EnqueuePacket(MarshalAgentDisplayCape(p.agentId, true))

	// GAME_SMSG_POST_PROCESS
	p.EnqueuePacket(MarshalPostProcess())
}

func (p *Player) sendPartySetup() {
	p.EnqueuePacket(MarshalUpdatePartySize(p.playerId, 1))
	p.EnqueuePacket(MarshalUnknown00b0(p.playerId, p.playerId))

	// GAME_SMSG_UPDATE_AGENT_PARTYSIZE - Duplicate!
	p.EnqueuePacket(MarshalUpdatePartySize(p.playerId, 1))

	// GAME_SMSG_PARTY_CREATE
	p.EnqueuePacket(MarshalPartyCreate(1))

	// GAME_SMSG_PARTY_PLAYER_ADD
	p.EnqueuePacket(MarshalPartyPlayerAdd(1, p.playerId))

	// GAME_SMSG_PARTY_MEMBER_STREAM_END
	p.EnqueuePacket(MarshalPartyMemberStreamEnd(1))

	// GAME_SMSG_PARTY_SEARCH_SEEK
	p.EnqueuePacket(MarshalPartySearchSeek())

	// GAME_SMSG_PARTY_SET_DIFFICULTY
	p.EnqueuePacket(MarshalPartySetDifficulty(false))

	// party something
	resp := GwPacket.NewOut(0x1b1)
	resp.Uint16(1)
	resp.Uint8(1)
	p.EnqueuePacket(resp)

	resp = GwPacket.NewOut(0x01bc)
	resp.Uint32(0)
	p.EnqueuePacket(resp)

	resp = GwPacket.NewOut(0x016d)
	resp.Uint8(0)
	p.EnqueuePacket(resp)
}

func (p *Player) sendCreateCharacterInstanceInfo() {
	p.log.Debug().Msg("sendCreateCharacterInstanceInfo")
	p.EnqueuePacket(MarshalInstancePlayerDataStart())
	p.EnqueuePacket(MarshalItemStreamCreate(1))

	// Need at least one item so that the response to Dye change requests is accepted without crash
	p.itemMgr.AddBag(20, 1) // backpack
	p.itemMgr.AddBag(9, 2)  // equipments

	p.EnqueuePacket(MarshalAgentUpdateAttributePoints(p.agentId, 0, 0))
	p.EnqueuePacket(MarshalPlayerUpdateProfession(p.agentId, 1, 0))
	p.EnqueuePacket(MarshalAgentAttrUpdateInt(64, p.agentId, 0))

	p.EnqueuePacket(MarshalInstancePlayerDataDone())
}

func (p *Player) sendWorldInstanceHead() {
	p.EnqueuePacket(MarshalInstancePlayerDataStart())
	p.EnqueuePacket(MarshalInstanceLoadPlayerName(p.name))
	p.EnqueuePacket(MarshalInstanceLoadInfo(p.playerId, p.connectedInstance.mapId, p.connectedInstance.IsExplorable(), 1, 0, false))
}

func (p *Player) sendWorldInstanceBody() {
	itemStreamId := 1
	resp := MarshalItemStreamCreate(itemStreamId)
	p.EnqueuePacket(resp)

	p.EnqueuePacket(MarshalActivateWeaponSet(itemStreamId))

	p.TransmitItems()

	p.EnqueuePacket(MarshalItemWeaponSet(itemStreamId, 1))
	p.EnqueuePacket(MarshalItemWeaponSet(itemStreamId, 2))
	p.EnqueuePacket(MarshalItemWeaponSet(itemStreamId, 3))

	p.EnqueuePacket(MarshalHeroInfo())
}

func (p *Player) sendUnlockedSkills() {
	resp := GwPacket.NewOut(0x001D)
	resp.Bytes([]byte{
		0x45, 0x00,
		0x06, 0x44, 0x80, 0xd6, 0xd0, 0x89, 0x14, 0x22, 0x38, 0x18, 0x31, 0x10, // 6
		0x61, 0x63, 0xcc, 0x09, 0x88, 0x88, 0x00, 0x22, 0x24, 0x02, 0x13, 0x00, 0x24, 0x01, 0x08, 0x50, // 14
		0x22, 0x08, 0x2c, 0x45, 0x00, 0x10, 0x20, 0x02, 0x21, 0x21, 0x2a, 0x02, 0x04, 0x04, 0x81, 0xb4, // 22
		0x00, 0x08, 0x00, 0x40, 0x43, 0x1c, 0x80, 0x08, 0x00, 0x85, 0x13, 0x40, 0x04, 0x08, 0x00, 0x00, // 30
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00, // 38
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 46
		0x00, 0x00, 0x00, 0x00, 0x30, 0x91, 0x80, 0x03, 0x00, 0x40, 0x20, 0x04, 0x09, 0x00, 0x00, 0x1c, // 54
		0x80, 0x00, 0x02, 0x00, 0x00, 0xc0, 0x48, 0x20, 0x40, 0x01, 0x00, 0x01, 0x15, 0x00, 0x68, 0x00, // 62
		0x00, 0x00, 0x00, 0x00, 0x90, 0x21, 0x18, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 69
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x0c, 0x42, 0xe7, 0xc1,
		0x2a, 0x04, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x01, 0x00, 0x00, 0x20, 0x10,
		0x40, 0x00, 0x90, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x30, 0x25, 0x94,
		0x6c, 0x1c, 0xc0, 0x88, 0x88, 0x85, 0x20, 0x00, 0x00, 0x04, 0x23, 0x58, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x04, 0x80, 0x80,
		0x0c, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
	})
	p.EnqueuePacket(resp)
	p.EnqueuePacket(MarshalSkillsUnlocked())
}

func (p *Player) sendUnlockedPvpHeroes() {
	p.EnqueuePacket(MarshalSetUnlockedHeroes([]uint16{}))
}

func (p *Player) sendQuestInfoSync() {
	p.EnqueuePacket(MarshalQuestsInfo(p.questBytes))
}

func (p *Player) sendMapsUnlockedSync() {

	/*
		0x202b,
		0x0006,
		0x202b,
		0x0006,
		0x202b,
		0x0006,
		0x202b,
		0x0006,
		0x202b,
		0x0006,
	*/
	/*
		uint32_t missions_completed_size;
		uint32_t missions_completed_buffer[32];
		uint32_t unlocked2_size;
		uint32_t unlocked2[32];
		uint32_t unlocked3_size;
		uint32_t unlocked3[32];
		uint32_t unlocked4_size;
		uint32_t unlocked4[32];
		uint32_t maps_unlocked_size;
		uint32_t maps_unlocked_buf[32];
	*/
	completedMissions := []uint32{}
	unk1 := []uint32{}
	unk2 := []uint32{}
	unk3 := []uint32{}
	maps := []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff}
	/*
		uint32_t real_index = map_id / 32;
		if (real_index >= array_size(&client->player_hero.maps_unlocked))
			goto leave;
		uint32_t flag = 1 << (map_id % 32);
		uint32_t val = array_at(&client->player_hero.maps_unlocked, real_index);
		ret = (val & flag) != 0;
	*/
	p.EnqueuePacket(MarshalMapsUnlocked2(completedMissions, unk1, unk2, unk3, maps))
	/*p.EnqueuePacket(MarshalMapsUnlocked([]byte{
		0x00, 0x00, // missions
		0x00, 0x00, // unk1
		0x00, 0x00, // unk2
		0x00, 0x00, // unk3
		0x1b, 0x00, // maps:27
		0x70, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x10, 0x00,
		0x02, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x14, 0x00, // 10th
		0xf6, 0xfd,
		0xdf, 0x17,
		0x00, 0x00,
		0x01, 0x00,
		0xb0, 0x00, // 15th
		0x00, 0x00,
		0x00, 0x00,
		0x18, 0x00,
		0x00, 0x18,
		0x10, 0x04,
		0x00, 0xff,
		0x07, 0x00,
		0xff, 0x41,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00, // 27th
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00, // 30th
		0x00, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0xe6, 0x67,
		0xc8, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00, // 40th
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x20, 0x07,
		0x3c, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x08, 0x10, // 50th
		0x00, 0x00,
		0x80, 0xff,
		0x1f, 0x00,
		0x00, 0x00, // 54th
	}))*/
}

func (p *Player) sendVanquishUpdate() {
	p.EnqueuePacket(MarshalVanquishProgress(0))
}

func (p *Player) sendDialogStuff() {
	// Maybe dialog related
	resp := GwPacket.NewOut(0x007b)
	resp.Uint32(1)
	p.EnqueuePacket(resp)

	// Maybe dialog related
	resp = GwPacket.NewOut(0x007c)
	resp.Uint32(1)
	p.EnqueuePacket(resp)
}

func (p *Player) sendAttributePointsRemaining() {
	p.EnqueuePacket(MarshalAgentUpdateAttributePoints(p.agentId, 0, 0))
}

func (p *Player) sendProfession() {
	p.EnqueuePacket(MarshalPlayerUpdateProfession(p.agentId, int(p.dbChar.ProfessionPrimary), int(p.dbChar.ProfessionSecondary)))
}

func (p *Player) sendUnlockedProfessions() {
	p.EnqueuePacket(MarshalPlayerUnlockedProfessions(p.agentId, 0))
}

func (p *Player) sendSkillbar() {
	part1 := make([]uint32, 8)
	part2 := make([]uint32, 8)
	p.EnqueuePacket(MarshalSkillbarUpdate(p.agentId, part1, part2))
}

func (p *Player) sendAttributeUpdateFloat(attributeId int) {
	p.EnqueuePacket(MarshalAgentAttrUpdateFloat(attributeId, p.agentId, 0.039600))
}

func (p *Player) sendCartographyData() {
	p.EnqueuePacket(MarshalCartographyDataStart())
	cartographyData := []uint32{
		0x001e0000,
		0x3a0221ff,
		0x39043a04,
		0x34093505,
		0x3707340a,
		0x36073806,
		0x320b340a,

		0x05070005,
		0x001b1102,
		0x07040514,
		0x05250203,
		0x02010805,
		0x0b080425,
		0x090b0325,
		0x033a0737,
		0x0094ffff,
		0x00000000,
		0x00000000,
		0x00cccccc,
	}
	p.EnqueuePacket(MarshalCartographyData(cartographyData))
}

func (p *Player) OnC2SChatMessage(payload ChatMessage) {
	if len(payload.message) <= 1 {
		return
	}
	p.log.Info().Int("ag", payload.agentId).Str("msg", payload.message).Msg("ChatMessage")

	// find channel by prefix char
	var channel = payload.message[0]
	var remainder = payload.message[1:]
	switch channel {
	case '!':
		p.connectedInstance.BroadcastLocalChat(p, remainder)
	case '/':
		// emote / command
		// extract next command word
		words := strings.Fields(remainder)
		if len(words) == 0 {
			p.SendChatWarning("Invalid command syntax")
			return
		}
		command := words[0]
		args := words[1:]
		// check whether it is an emote command
		if emote, exists := GetEmoteByCommand(command); exists {
			p.connectedInstance.BroadcastGeneric(MarshalEmote(p.playerId, p.agentId, emote))
			return
		}
		// not an emote, check for other commands
		HandleCommand(p, command, remainder, args)
	}
}

func (p *Player) sendAgentDespawned(agent *Agent) {
	p.EnqueuePacket(MarshalAgentDespawned(agent.agentId))
}
