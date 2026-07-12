package GameService

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func init() {
	log = zerolog.New(zerolog.NewConsoleWriter())
	log = log.Level(zerolog.DebugLevel)
	log = log.With().Timestamp().Logger()
}

type agentSpawnInfo struct {
	Name       string     `json:"name"`
	Level      int        `json:"level"`
	SpawnPoint [3]float32 `json:"spawn_point"`
}

type instanceDefinition struct {
	DebugName   string           `json:"debug_name"`
	Explorable  bool             `json:"explorable"`
	MapFileId   int              `json:"map_file_id"`
	PartySize   int              `json:"party_size"`
	Agents      []agentSpawnInfo `json:"agents"`
	SpawnPoints [][]float32      `json:"spawn_points,omitempty"`
}

type agentDefinition struct {
	Name               string  `json:"name"`
	EncName            string  `json:"enc_name"`
	ModelId            int     `json:"model_id"`
	AllegianceFlags    int     `json:"allegiance_flags"`
	Speed              float32 `json:"speed"`
	Profession         int     `json:"profession"`
	FileId             int     `json:"file_id"`
	UnkPropertiesBytes string  `json:"unk_properties_bytes"`
	DefinitionIndex    int
}

var instanceDefinitions = struct {
	Instances map[string]instanceDefinition `json:"instances"`
	Agents    map[string]agentDefinition    `json:"agents"`
}{
	Instances: make(map[string]instanceDefinition, 0),
	Agents:    make(map[string]agentDefinition, 0),
}

func LoadInstanceDefinitionsFromDisk() error {
	file, err := os.Open("gameservice/instance_definitions.json")
	if err != nil {
		return fmt.Errorf("failed to load instance definitions file: %w", err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&instanceDefinitions); err != nil {
		return fmt.Errorf("failed to parse instance definitions: %w", err)
	}
	log.Info().Int("count", len(instanceDefinitions.Instances)).Msg("loaded instance definitions from disk")
	log.Info().Int("count", len(instanceDefinitions.Agents)).Msg("loaded agent definitions from disk")

	// Annotate NPC agent definitions with an index
	index := 0
	for name := range instanceDefinitions.Agents {
		def := instanceDefinitions.Agents[name]
		def.DefinitionIndex = index
		instanceDefinitions.Agents[name] = def
		index++
	}

	// Now start up all persistent instances:
	for mapIdStr, definition := range instanceDefinitions.Instances {
		mapId, err := strconv.Atoi(mapIdStr)
		if err != nil {
			panic(fmt.Errorf("bad map id %s: %w", mapIdStr, err))
		}
		if definition.Explorable {
			continue
		}

		inst := NewInstance(mapId, definition)
		InstanceManager.AddInstance(&inst)
	}
	log.Info().Int("count", len(InstanceManager.instances)).Msg("persistent instances created")
	return nil
}

type instanceManager struct {
	instances map[uint64]*Instance
}

var InstanceManager = instanceManager{
	instances: make(map[uint64]*Instance),
}

func (im *instanceManager) GetOrCreateInstanceByMapId(mapId int) *Instance {
	// Check definition for mapId
	definition, ok := instanceDefinitions.Instances[strconv.Itoa(mapId)]
	if !ok {
		log.Error().Int("mapId", mapId).Msg("missing instance definition")
		return nil
	}
	var inst Instance
	if !definition.Explorable {
		// Public, persistent instance
		existingInst, hasExistingInst := im.GetInstanceByMapId(mapId)
		if !hasExistingInst {
			log.Error().Int("mapId", mapId).Msg("missing persistent instance")
			return nil
		}
		return existingInst
	}
	// Private instance -- create one now:
	inst = NewInstance(mapId, definition)
	im.AddInstance(&inst)
	return &inst
}

func (im *instanceManager) GetInstanceByMapId(mapId int) (*Instance, bool) {
	for _, inst := range im.instances {
		if inst.mapId == mapId {
			return inst, true
		}
	}
	return nil, false
}

func (im *instanceManager) AddInstance(instance *Instance) {
	im.instances[instance.uuid] = instance
	go instance.MainLoop()
	go instance.MovementTickLoop()
}

type Instance struct {
	uuid                   uint64
	players                []*Player
	mapId                  int
	definition             instanceDefinition
	alive                  bool
	agents                 []Agent
	gracefulShutdownSignal chan bool
	forceShutdownSignal    chan bool
	log                    zerolog.Logger
}

func (inst *Instance) TransmitAgentDespawned(agent *Agent) {
	for _, other := range inst.players {
		other.sendAgentDespawned(agent)
	}
}

func (inst *Instance) RemovePlayer(player *Player) {
	inst.TransmitAgentDespawned(&player.Agent)
	for i, v := range inst.players {
		if v == nil {
			continue
		}
		if player.uuid == v.uuid {
			// Remove the element by re-slicing
			inst.players = slices.Delete(inst.players, i, i+1)
		}
	}
	inst.log.Debug().Uint64("playerUuid", player.uuid).Msg("player removed from instance")
	if inst.definition.Explorable && len(inst.players) == 0 {
		inst.log.Debug().Msg("explorable instance shutting down due to inactivity")
		inst.gracefulShutdownSignal <- true
	}
}

func NewInstance(mapId int, definition instanceDefinition) (i Instance) {
	i = Instance{
		definition:             definition,
		uuid:                   rand.Uint64(),
		mapId:                  mapId,
		alive:                  true,
		agents:                 make([]Agent, 0),
		gracefulShutdownSignal: make(chan bool, 1),
		forceShutdownSignal:    make(chan bool, 1),
	}
	i.log = log.With().Uint64("uuid", i.uuid).Int("mapId", i.mapId).Logger()
	if i.definition.Explorable {
		i.log.Debug().Msg("created a new explorable instance")
	}

	// Set up agents!
	for _, agentToSpawn := range i.definition.Agents {
		agentDefinition, ok := instanceDefinitions.Agents[agentToSpawn.Name]
		if !ok {
			log.Error().Str("name", agentToSpawn.Name).Msg("missing definition for agent")
		}
		ag := Agent{
			agentId:             i.NextFreeAgentId(),
			definitionIndex:     agentDefinition.DefinitionIndex,
			name:                agentDefinition.Name,
			posX:                agentToSpawn.SpawnPoint[0],
			posY:                agentToSpawn.SpawnPoint[1],
			plane:               int(agentToSpawn.SpawnPoint[2]),
			facingX:             1.0,
			facingY:             0.0,
			speed:               agentDefinition.Speed,
			modelId:             agentDefinition.ModelId,
			allegianceFlags:     agentDefinition.AllegianceFlags,
			encName:             agentDefinition.EncName,
			primaryProfession:   agentDefinition.Profession,
			secondaryProfession: 0,
			level:               agentToSpawn.Level,
			fileId:              agentDefinition.FileId,
			unkPropertiesBytes:  agentDefinition.UnkPropertiesBytes, // Really what is this? you can set to all 0 and it seems the same?
		}
		i.agents = append(i.agents, ag)
		log.Info().Str("name", agentToSpawn.Name).Int("agentId", ag.agentId).Msg("added agent!")
	}
	return
}

func (i Instance) IsCharCreationInstance() bool {
	return i.mapId == 0
}

func (i *Instance) MainLoop() {
	for {
		select {
		case <-i.gracefulShutdownSignal:
			i.log.Debug().Msg("graceful shutdown")
			return
		case <-i.forceShutdownSignal:
			i.log.Debug().Msg("force shutdown")
			i.alive = false
			return
		default:
			time.Sleep(time.Second * 5)
			for _, player := range i.players {
				if player.conn.closed {
					continue
				}
				player.EnqueuePacket(MarshalServerPingRequest(30, 491)) // dont know what these values mean
			}
		}
	}
}

func (i *Instance) MovementTickLoop() {
	for i.alive {
		time.Sleep(time.Millisecond * 500)
		for _, player := range i.players {
			if player.conn.closed {
				continue
			}
			player.EnqueuePacket(MarshalAgentMovementTick(500))
		}
	}
}

func (i *Instance) NextFreeAgentId() int {
	return len(i.agents) + 1
}
func (i *Instance) NextFreePlayerId() int {
	return len(i.players) + 10
}

func (i *Instance) NextSpawnPoint() (x, y float32, plane int) {
	nSpawnPoints := len(i.definition.SpawnPoints)
	if nSpawnPoints == 0 {
		panic(fmt.Errorf("instance for map id %d has no spawn points", i.mapId))
	}
	spawnPoint := i.definition.SpawnPoints[0]
	x = spawnPoint[0]
	y = spawnPoint[1]
	plane = int(spawnPoint[2])
	return
}

func parseUTF16HexString(s string) (string, error) {
	// Split the input string by space
	parts := strings.Fields(s)

	// Create a slice of uint16 to store code units
	var codeUnits []uint16
	for _, part := range parts {
		val, err := strconv.ParseUint(part, 16, 16)
		if err != nil {
			return "", fmt.Errorf("invalid hex code unit %q: %w", part, err)
		}
		codeUnits = append(codeUnits, uint16(val))
	}

	// Decode UTF-16 code units into runes
	runes := utf16.Decode(codeUnits)

	return string(runes), nil
}

func convertEncName(in string) []byte {
	// "2d9e f878 bdbf 12e7"
	conv := []byte{}
	fields := strings.Fields(in)
	for _, word := range fields {
		// Parse the 4-digit hex word into a uint16
		val, err := strconv.ParseUint(word, 16, 16)
		if err != nil {
			panic(fmt.Errorf("invalid hex word %q: %w", word, err))
		}
		conv = append(conv, byte(val&0xff), byte(val>>8))
	}
	return conv
}

func (i *Instance) AddPlayer(player *Player) {
	player.agentId = i.NextFreeAgentId()
	player.playerId = i.NextFreePlayerId()
	i.players = append(i.players, player)
	i.agents = append(i.agents, player.Agent)
	fmt.Printf("%s added to instance.\n", player.name)
	fmt.Printf("%d players in instance:\n", len(i.players))
	for i, v := range i.players {
		fmt.Printf("  #%d = PlayerID=%d AgentID=%d Name=%s\n", i, v.playerId, v.agentId, v.name)
	}
	player.EnqueuePacket(MarshalInstanceLoadHead())
	if i.IsCharCreationInstance() {
		player.EnqueuePacket(MarshalCharCreationStart())
		player.conn.sendCreateCharacterInstanceInfo()
	} else {
		player.posX, player.posY, player.plane = i.NextSpawnPoint()
		player.conn.sendWorldInstanceHead()
		player.conn.sendWorldInstanceBody()
		player.EnqueuePacket(MarshalUpdateCurrentMapId(i.mapId))
		player.EnqueuePacket(MarshalReadyForMapSpawn())
		player.EnqueuePacket(MarshalInstanceManifestDone(0, 1, 0))

		i.TransmitPlayerToOthers(player)
	}
}

func contains(slice []int, val any) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func randomFloatAround(start, rangeVal float32) float32 {
	offset := (rand.Float32() * 2 * rangeVal) - rangeVal
	return start + offset
}

func (i *Instance) SendActiveAgents(to *Player) {

	// Let's send agent info now.
	transmittedDefinitions := make([]int, 0)
	for _, ag := range i.agents {
		if ag.isPlayer {
			continue
		}

		// NOTE: UpdateNPCProperties and UpdateNPCModel are only transmitted for the first instance of that NPC definition
		// It doesn't look like the client cares what goes into the NpcID property?
		if !contains(transmittedDefinitions, ag.definitionIndex) {
			// Original code was this:
			//agentType := (0x2000 << 16)|(npcIdFromPacketCapture & 0xffff)
			to.EnqueuePacket(MarshalAgentUpdateNPCProperties(ag.definitionIndex, ag.fileId, ag.primaryProfession, ag.level, convertEncName(ag.unkPropertiesBytes)))
			to.EnqueuePacket(MarshalAgentUpdateNPCModel(ag.definitionIndex, ag.modelId))
			transmittedDefinitions = append(transmittedDefinitions, ag.definitionIndex)
		}

		to.EnqueuePacket(MarshalAgentUpdateNPCName(ag.agentId, convertEncName(ag.encName)))
		to.EnqueuePacket(MarshalAgentInitialEffects(ag.agentId, 0))
		// for allegiance:
		// Player has       0x706c6179
		// normal NPC has   0x706C6179
		// blocking NPC has 0x6e6f6e63
		agentType := (0x2000 << 16) | ag.definitionIndex
		to.EnqueuePacket(MarshalAgentSpawned(
			ag.agentId,
			agentType,
			1,
			9,
			ag.posX, ag.posY, ag.plane,
			ag.facingX, ag.facingY,
			ag.speed,
			ag.allegianceFlags,
		))
		i.log.Info().Int("agentId", ag.agentId).Int("ToAgId", to.agentId).Int("ToPlayerId", to.playerId).Msg("Transmitted Agent")
	}
	i.TransmitOtherPlayersToPlayer(to)
}

func (i *Instance) TransmitOtherPlayersToPlayer(to *Player) {
	for _, other := range i.players {
		if other.playerId == to.playerId {
			continue
		}
		i.TransmitPlayer(to, other)
	}
}

func (i *Instance) TransmitPlayerToOthers(player *Player) {
	for _, other := range i.players {
		if other.playerId == player.playerId {
			continue
		}
		i.TransmitPlayer(other, player)
	}
}

func (i *Instance) TransmitPlayer(to *Player, other *Player) {
	to.EnqueuePacket(MarshalAgentCreatePlayer(other.playerId, other.agentId, int(other.dbChar.AppearanceBits), other.name))
	to.EnqueuePacket(MarshalAgentUpdateProfession(other.agentId, other.primaryProfession, other.secondaryProfession))
	to.EnqueuePacket(MarshalAgentAttrUpdateInt(36, other.agentId, other.level))
	to.EnqueuePacket(MarshalAgentInitialEffects(other.agentId, 0))
	agentType := 0x30000000
	agentType |= other.playerId
	to.EnqueuePacket(MarshalAgentSpawned(
		other.agentId,
		agentType,
		1,
		5,
		other.posX,
		other.posY,
		0,
		other.facingX,
		other.facingY,
		other.speed,
		other.allegianceFlags,
	))
	// What's this?
	to.EnqueuePacket(MarshalAgentAttrUpdateInt(30, other.agentId, other.playerId))
}

func (i *Instance) UpdateRequestedPlayerPos(player *Player, x float32, y float32) {
	// The player requested a new position -- for now just update the instance definition and transmit movement update to everyone.
	player.posX = x
	player.posY = y
	for _, other := range i.players {
		other.EnqueuePacket(MarshalAgentUpdatePosition(player.agentId, x, y, 0))
	}
}

func (i *Instance) BroadcastLocalChat(from *Player, message string) {
	packet := MarshalChatMessageCore(message)
	packet.Merge(MarshalChatMessageLocal(from.playerId, 3))

	for _, other := range i.players {
		other.EnqueuePacket(packet)
	}

	// TODO: if nobody else in zone, send "No one hears you..."
	/*
		// No one hears you:
		unk := GwPacket.NewOut(0x5C)
		unk.Uint16(1)
		unk.Uint16(0x087b)
		packet.Merge(unk)
		unk2 := GwPacket.NewOut(0x5D)
		unk2.Uint16(1)
		unk2.Uint8(13)
		packet.Merge(MarshalChatMessageServer(13))
	*/
}
