package GameService

import (
	"encoding/json"
	"fmt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
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

type HexStr int

func (h *HexStr) UnmarshalJSON(data []byte) error {
	// First unmarshal the JSON string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Optional: allow both "0x..." and plain hex
	s = strings.TrimPrefix(strings.ToLower(s), "0x")

	v, err := strconv.ParseInt(s, 16, 0)
	if err != nil {
		return err
	}

	*h = HexStr(v)
	return nil
}

type agentSpawnInfo struct {
	Name       string     `json:"name"`
	Level      int        `json:"level"`
	SpawnPoint [3]float32 `json:"spawn_point"`
}

type instanceDefinition struct {
	Name        string           `json:"name"`
	Explorable  bool             `json:"explorable"`
	MapFileId   HexStr           `json:"map_file_id"`
	PartySize   int              `json:"party_size,omitempty"`
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
	Instances map[int]instanceDefinition `json:"instances"`
	Agents    map[string]agentDefinition `json:"agents"`
}{
	Instances: make(map[int]instanceDefinition, 0),
	Agents:    make(map[string]agentDefinition, 0),
}

func LoadInstanceDefinitionsFromDisk() error {
	file, err := os.Open("data/instance_definitions.json")
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
	for mapId, definition := range instanceDefinitions.Instances {
		if definition.Explorable {
			continue
		}

		nSpawnPoints := len(definition.SpawnPoints)
		if nSpawnPoints == 0 {
			log.Error().Int("mapId", mapId).Msg("map definition has no spawn points")
		}

		inst := NewInstance(mapId, definition)
		InstanceManager.AddInstance(&inst)
	}
	log.Info().Int("count", len(InstanceManager.instances)).Msg("persistent instances created")
	return nil
}

func GetMapIdForName(name string) (int, bool) {
	for mapId, definition := range instanceDefinitions.Instances {
		if definition.Name == name {
			return mapId, true
		}
	}
	return 0, false
}

func HasInstanceDefinitionForMapId(mapId int) bool {
	if mapId == 0 {
		return false // char creation instance is not a real map
	}
	_, ok := instanceDefinitions.Instances[mapId]
	return ok
}

type instanceManager struct {
	instances map[uint64]*Instance
	mu        sync.RWMutex
}

var InstanceManager = instanceManager{
	instances: make(map[uint64]*Instance),
	mu:        sync.RWMutex{},
}

func (im *instanceManager) GetOrCreateInstanceByMapId(mapId int) (*Instance, error) {
	// Check definition for mapId
	definition, ok := instanceDefinitions.Instances[mapId]
	if !ok {
		return nil, fmt.Errorf("missing instance definition for map id %d", mapId)
	}
	var inst Instance
	if !definition.Explorable {
		// Public, persistent instance
		existingInst, hasExistingInst := im.GetInstanceByMapId(mapId)
		if !hasExistingInst {
			log.Error().Int("mapId", mapId).Msg("missing persistent instance")
			return nil, fmt.Errorf("missing persistent instance for non-explorable map id %d", mapId)
		}
		return existingInst, nil
	}
	// Private instance -- create one now:
	inst = NewInstance(mapId, definition)
	im.AddInstance(&inst)
	return &inst, nil
}

func (im *instanceManager) BroadcastPacketToAllPlayers(packet GwPacket.Out) {
	im.mu.Lock()
	defer im.mu.Unlock()
	for _, inst := range im.instances {
		inst.BroadcastGeneric(packet)
	}
}

func (im *instanceManager) NumPlayersOnline() int {
	im.mu.Lock()
	defer im.mu.Unlock()
	x := 0
	for _, inst := range im.instances {
		x += len(inst.players)
	}
	return x
}

func (im *instanceManager) GetInstanceByMapId(mapId int) (*Instance, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	for _, inst := range im.instances {
		if inst.mapId == mapId {
			return inst, true
		}
	}
	return nil, false
}

func (im *instanceManager) AddInstance(instance *Instance) {
	im.mu.Lock()
	im.instances[instance.uuid] = instance
	im.mu.Unlock()
	go instance.MainLoop()
	go instance.MovementTickLoop()
}

type Instance struct {
	uuid                   uint64
	tag                    uint32
	players                []*Player
	mapId                  int
	definition             instanceDefinition
	alive                  bool
	agents                 []Agent
	gracefulShutdownSignal chan bool
	forceShutdownSignal    chan bool
	log                    zerolog.Logger
	mu                     *sync.RWMutex
}

func (inst *Instance) TransmitAgentDespawned(agent *Agent) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	for _, other := range inst.players {
		other.sendAgentDespawned(agent)
	}
}

func (inst *Instance) RemovePlayer(player *Player) {
	inst.mu.Lock()
	removed := false
	for i, v := range inst.players {
		if v == nil {
			continue
		}
		if player.uuid == v.uuid {
			inst.players = slices.Delete(inst.players, i, i+1)
			removed = true
			break // stop iterating over a slice we just mutated
		}
	}
	remaining := len(inst.players)
	inst.mu.Unlock()
	if !removed {
		return
	}
	// so the departing player doesn't get sent their own despawn.
	inst.TransmitAgentDespawned(&player.Agent)

	inst.log.Debug().Uint64("playerUuid", player.uuid).Msg("player removed from instance")
	if inst.definition.Explorable && remaining == 0 {
		inst.log.Debug().Msg("explorable instance shutting down due to inactivity")
		inst.gracefulShutdownSignal <- true
	}
}

func NewInstance(mapId int, definition instanceDefinition) (i Instance) {
	i = Instance{
		definition:             definition,
		uuid:                   rand.Uint64(),
		tag:                    rand.Uint32(),
		mapId:                  mapId,
		alive:                  true,
		agents:                 make([]Agent, 0),
		gracefulShutdownSignal: make(chan bool, 1),
		forceShutdownSignal:    make(chan bool, 1),
		mu:                     &sync.RWMutex{},
	}
	i.log = log.With().Uint64("uuid", i.uuid).Int("mapId", i.mapId).Logger()
	if i.definition.Explorable {
		i.log.Debug().Msg("created a new explorable instance")
	}

	// Set up agents!
	for _, agentToSpawn := range i.definition.Agents {
		agentDefinition, ok := instanceDefinitions.Agents[agentToSpawn.Name]
		if !ok {
			log.Error().Int("mapId", mapId).Str("name", agentToSpawn.Name).Msg("missing definition for agent")
			continue
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
		//log.Info().Str("name", agentToSpawn.Name).Int("agentId", ag.agentId).Msg("added agent!")
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
			i.mu.Lock()
			i.alive = false
			i.mu.Unlock()
			return
		case <-i.forceShutdownSignal:
			i.log.Debug().Msg("force shutdown")
			i.mu.Lock()
			i.alive = false
			i.mu.Unlock()
			return
		default:
			time.Sleep(time.Second * 5)
			i.mu.RLock()
			for _, player := range i.players {
				if player.conn.closed {
					continue
				}
				player.EnqueuePacket(MarshalServerPingRequest(30, 491)) // dont know what these values mean
			}
			i.mu.RUnlock()
		}
	}
}

func (i *Instance) isAlive() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.alive
}

func (i *Instance) MovementTickLoop() {
	for i.isAlive() {
		time.Sleep(time.Millisecond * 500)
		i.mu.RLock()
		for _, player := range i.players {
			if player.conn.closed {
				continue
			}
			player.EnqueuePacket(MarshalAgentMovementTick(500))
		}
		i.mu.RUnlock()
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

func (i *Instance) NextFreeAgentId() int {
	return len(i.agents) + 1
}
func (i *Instance) NextFreePlayerId() int {
	return len(i.players) + 1
}

func (i *Instance) NextSpawnPoint() (x, y float32, plane int) {
	nSpawnPoints := len(i.definition.SpawnPoints)
	// Special case for dev:
	if nSpawnPoints == 0 {
		x = 0.0
		y = 0.0
		plane = 0
		return
	}
	// Choose a random spawn point:
	randIndex := rand.Intn(nSpawnPoints)
	spawnPoint := i.definition.SpawnPoints[randIndex]
	x = randomFloatAround(spawnPoint[0], 100.0)
	y = randomFloatAround(spawnPoint[1], 100.0)
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
	i.mu.Lock()
	player.agentId = i.NextFreeAgentId()
	player.playerId = i.NextFreePlayerId()
	i.players = append(i.players, player)
	i.agents = append(i.agents, player.Agent)
	i.mu.Unlock()
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
		player.EnqueuePacket(MarshalInstanceManifestDone(0, i.mapId, 0))
		player.SendWelcomeChatMessage()

		i.TransmitPlayerToOthers(player)
	}
}

func (i *Instance) SendActiveAgents(to *Player) {
	i.mu.RLock()
	defer i.mu.RUnlock()

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
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, other := range i.players {
		if other.playerId == to.playerId {
			continue
		}
		i.TransmitPlayer(to, other)
	}
}

func (i *Instance) TransmitPlayerToOthers(player *Player) {
	i.mu.RLock()
	defer i.mu.RUnlock()
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
		other.plane,
		other.facingX,
		other.facingY,
		other.speed,
		other.allegianceFlags,
	))
	// What's this?
	to.EnqueuePacket(MarshalAgentAttrUpdateInt(30, other.agentId, other.playerId))
}

func (i *Instance) UpdateRequestedPlayerPos(player *Player, x float32, y float32) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	found := false
	for _, cur := range i.players {
		if cur.playerId == player.playerId {
			found = true
			break
		}
	}
	if !found {
		i.log.Warn().Msg("refusing to update player pos for a player not in this instance")
		return
	}
	// The player requested a new position -- for now just update the instance definition and transmit movement update to everyone.
	player.posX = x
	player.posY = y
	for _, other := range i.players {
		other.EnqueuePacket(MarshalAgentUpdatePosition(player.agentId, x, y, player.plane))
	}
}

func (i *Instance) BroadcastGeneric(packet GwPacket.Out) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	for _, other := range i.players {
		other.EnqueuePacket(packet)
	}
}

func (i *Instance) BroadcastLocalChat(from *Player, message string) {
	packet := MarshalChatMessageCore(message)
	packet.Merge(MarshalChatMessageLocal(from.playerId, 3))
	i.BroadcastGeneric(packet)

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

func (i *Instance) GetTag() uint32 {
	return i.tag
}

func (i *Instance) TransferPlayerToNewMap(player *Player, newMapId int) error {
	// TODO: check valid map
	// TODO: check player has map unlocked
	// TODO: check same continent
	// TODO: check they are party leader
	// TODO: also transport party

	inst, err := InstanceManager.GetOrCreateInstanceByMapId(newMapId)
	if inst == nil || err != nil {
		// something went wrong - decline connection
		player.log.Error().Err(err).Msg("unable to create instance")
		player.Disconnect()
		return nil
	}

	// Generate a security token for the transfer
	instanceTag := inst.GetTag()
	securityTag := GenerateConnectionTokenForInstance(instanceTag)

	// Next, remove player from current instance
	i.RemovePlayer(player)

	// Next, send packets to client
	region := 1
	player.conn.EnqueuePacket(MarshalTransferGameServerInfo([]byte{
		0x02, 0x00, // AF_INET
		0x17, 0xe0, // Port 6112
		0xc0, 0xa8, 0x01, 0x7c, // 192.168.1.124
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, int(instanceTag), region, newMapId, i.IsExplorable(), int(securityTag)))
	player.conn.EnqueuePacket(MarshalUpdateCurrentMapId(newMapId))
	// Put in new instance:
	player.connectedInstance = inst
	err = db.SetLastOutpostForChar(player.dbChar.ID, uint16(newMapId))
	if err != nil {
		player.log.Error().Err(err).Msg("unable to update last outpost")
		return err
	}
	player.log.Info().Msg("Switched instances and synced db")
	return nil
}

func (i *Instance) IsExplorable() bool {
	return i.definition.Explorable
}
