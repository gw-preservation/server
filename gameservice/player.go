package GameService

import (
	"bytes"
	"fmt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	"math/rand"
	"strings"

	"github.com/rs/zerolog"
)

type Player struct {
	Agent
	playerId           int
	bags               []db.Bag
	conn               *GSConn
	questBytes         []byte
	connectedInstance  *Instance
	log                zerolog.Logger
	readyForAgentTicks bool // TODO: refactor into LoadingState / ReadyState
	xp                 int

	dbAcc  db.Account
	dbChar db.Character
}

func NewPlayer(conn *GSConn, logCtx zerolog.Logger) Player {
	p := Player{
		conn:               conn,
		questBytes:         make([]byte, 0),
		readyForAgentTicks: false,
	}
	p.allegianceFlags = 0x706c6179
	p.uuid = rand.Uint64()
	p.log = logCtx.With().Uint64("uuid", p.uuid).Logger()
	p.isPlayer = true
	return p
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

func (p *Player) OnC2SVerifyConnection(payload VerifyClientConnection) {
	// We should validate now, to check the request is valid
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
				p.dbChar = char
				verified = true
				break
			}
		}
		if !verified {
			p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("no such character")
			p.Disconnect()
			return
		}
		// Merge from DB data:
		p.name = p.dbChar.Name
		p.primaryProfession = int(p.dbChar.ProfessionPrimary)
		p.secondaryProfession = int(p.dbChar.ProfessionSecondary)
		p.level = int(p.dbChar.Level)
		p.xp = int(p.dbChar.XP)
		// Bags
		bags, ok := db.GetBagsForCharacterByID(p.dbChar.ID)
		if !ok {
			p.log.Error().Uint64("id", p.dbChar.ID).Msg("failed to get bags for character")
		}
		p.bags = bags
	}

	// TODO: Here we should verify the map is adjacent to the LastOutpostID if its explorable!

	// Hook client up to an instance
	inst, err := InstanceManager.GetOrCreateInstanceByMapId(payload.mapId)
	if inst == nil || err != nil {
		// something went wrong - decline connection
		p.log.Error().Err(err).Msg("unable to create instance")
		p.Disconnect()
		return
	}
	p.connectedInstance = inst

	p.log.Debug().Int("mapId", payload.mapId).Msg("VerifyClientConnection")
}

func (p *Player) OnUserDisconnected() {

}

func (p *Player) OnC2SUpdateProfessionChoice(payload UpdateProfessionChoice) {
	p.log.Debug().
		Int("profession", payload.professionId).
		Bool("isPvE", payload.isPvE).
		Msg("UpdateProfessionChoice")

	p.EnqueuePacket(MarshalPvPItemsEnd())
	p.EnqueuePacket(MarshalPlayerUpdateProfession(1, payload.professionId, 0))

	p.EnqueuePacket(MarshalItemSetProfession(1, payload.professionId))

}

func (p *Player) OnC2SDyeEquipment(payload DyeEquipment) {
	//p.log.Info().Int("Prof", p.primaryProfession).Msg("C2SDyeEquipment")
	//p.EnqueuePacket(MarshalItemSetProfession(1, 5))
	resp := GwPacket.NewOut(0x15A)
	resp.Uint32(1)
	resp.Uint32(1)
	p.EnqueuePacket(resp)
}

func (p *Player) sendInstanceLoadSpawnPoint() {
	p.log.Debug().Msg("InstanceLoadRequestSpawnPoint")
	inst := *p.connectedInstance
	p.EnqueuePacket(MarshalInstanceLoadSpawnPoint(inst.definition.MapFileId, p.posX, p.posY, p.plane, false, []byte{0xcd, 0x49, 0x03, 0xcc, 0x17, 0xa7, 0xdb, 0x01}))
}

func (p *Player) sendInstanceLoadRequestPlayers(payload InstanceLoadRequestPlayers) {
	p.log.Debug().Hex("unkBlob", payload.unk1).Int("playerAgentId", p.agentId).Msg("InstanceLoadRequestPlayers")
	// Sync skill info
	p.sendUnlockedSkills()
	p.sendSkillbar()
	// Sync attribute points
	p.sendAttributePointsRemaining()
	// Sync profession info
	p.sendProfession()
	p.sendUnlockedProfessions()

	p.sendUnlockedPvpHeroes()
	p.EnqueuePacket(GwPacket.NewOut(0x001b))
	// Sync quest info
	p.sendQuestInfoSync()
	// Sync unlocked maps / cartography data
	p.sendMapsUnlockedSync()
	p.sendCartographyData()
	// Sync vanquish info
	//p.sendVanquishUpdate()
	p.EnqueuePacket(MarshalInstanceLoaded())
	//p.sendDialogStuff()
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

	// GAME_SMSG_AGENT_DISPLAY_CAPE
	p.EnqueuePacket(MarshalAgentDisplayCape(p.agentId, true))

	// GAME_SMSG_POST_PROCESS
	p.EnqueuePacket(MarshalPostProcess())

	// party info

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
	resp = GwPacket.NewOut(0x1b1)
	resp.Uint16(1)
	resp.Uint8(1)
	p.EnqueuePacket(resp)

	resp = GwPacket.NewOut(0x01bc)
	resp.Uint32(0)
	p.EnqueuePacket(resp)

	resp = GwPacket.NewOut(0x016d)
	resp.Uint8(0)
	p.EnqueuePacket(resp)

	// GAME_SMSG_INSTANCE_LOAD_FINISH
	p.EnqueuePacket(MarshalInstanceLoadFinish())

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
		// check whether it is an emote command
		if emote, exists := GetEmoteByCommand(command); exists {
			p.connectedInstance.BroadcastGeneric(p, MarshalEmote(p.playerId, p.agentId, emote))
			return
		}
		// not an emote, check for other commands
		switch command {
		case "motd":
			p.EnqueuePacket(MarshalMessageOfTheDay("\u0155"))
		case "gv":
			if len(words) < 3 {
				p.SendChatWarning("Usage: /gv <typ> <value>")
				return
			}
			var msgType int
			nParsed, err := fmt.Sscanf(words[1], "%d", &msgType)
			if nParsed == 0 || err != nil {
				p.SendChatWarning("Usage: /gv <typ> <value>")
				return
			}
			var value int
			nParsed, err = fmt.Sscanf(words[2], "%d", &value)
			if nParsed == 0 || err != nil {
				p.SendChatWarning("Usage: /gv <typ> <value>")
				return
			}
			p.log.Info().Int("msgType", msgType).Int("value", value).Msg("Sending GenericValue message")
			// 6 (AddEffect):
			//   24 = Black effect from eyes
			//   20 = Blue swirly
			//   19 = Orb thingy
			//   18 = Unknown thingy
			//   17 = Big blue ring
			//   15 = Blue swirly
			p.conn.EnqueuePacket(MarshalAgentAttrUpdateInt(msgType, p.agentId, value))
		case "color":
			p.SendChatColorTest()
		case "travel":
			if len(words) < 2 {
				p.SendChatWarning("Usage: /travel <mapId> or /travel \"<map_debug_name>\"")
				return
			}

			var newMapId int
			nParsed, err := fmt.Sscanf(words[1], "%d", &newMapId)
			if nParsed == 0 || err != nil {
				// maybe it's a name instead of an ID
				var ok bool
				newMapId, ok = GetMapIdForDebugName(words[1])
				if !ok || newMapId == 0 {
					p.log.Error().Err(err).Msg("failed to find map by id or debug name")
					return
				}
			}
			p.log.Info().Int("newMapId", newMapId).Msg("travel command")
			// Is it a valid map?
			if !HasInstanceDefinitionForMapId(newMapId) {
				p.SendChatWarning(fmt.Sprintf("Map ID %d is not valid", newMapId))
				return
			}
			// Transfer player to new map
			err = p.connectedInstance.TransferPlayerToNewMap(p, newMapId)
			if err != nil {
				p.log.Error().Err(err).Int("newMapId", newMapId).Msg("failed to transfer player to new map")
				return
			}

		default:
			p.SendChatWarning(fmt.Sprintf("Unknown command: %s", remainder))
		}
	}
}

func (p *Player) sendAgentDespawned(agent *Agent) {
	p.EnqueuePacket(MarshalAgentDespawned(agent.agentId))
}
