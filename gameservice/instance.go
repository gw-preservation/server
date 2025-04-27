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

type instanceDefinition struct {
	DebugName   string   `json:"debug_name"`
	Explorable  bool     `json:"explorable"`
	MapFileId   int      `json:"map_file_id"`
	PartySize   int      `json:"party_size"`
	AgentNames  []string `json:"agents"`
	SpawnPoints [][]int  `json:"spawn_points,omitempty"`
}

type agentDefinition struct {
	EncName            string  `json:"enc_name"`
	ModelId            int     `json:"model_id"`
	AgentType          int     `json:"agent_type"`
	ModelType          int     `json:"model_type"`
	Speed              float32 `json:"speed"`
	SpawnPoint         [3]int  `json:"spawn_point"`
	Profession         int     `json:"profession"`
	Level              int     `json:"level"`
	FileId             int     `json:"file_id"`
	UnkPropertiesBytes string  `json:"unk_properties_bytes"`
}

var instanceDefinitions = struct {
	Instances map[string]instanceDefinition `json:"instances"`
	Agents    map[string]agentDefinition    `json:"agents"`
}{
	Instances: make(map[string]instanceDefinition, 0),
	Agents:    make(map[string]agentDefinition, 0),
}

func LoadInstanceDefinitionsFromDisk() {
	file, err := os.Open("gameservice/instance_definitions.json")
	if err != nil {
		panic(fmt.Sprintf("failed to load instance definitions: %v", err))
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&instanceDefinitions); err != nil {
		panic(fmt.Sprintf("failed to load instance definitions: %v", err))
	}
	log.Info().Int("count", len(instanceDefinitions.Instances)).Msg("loaded instance definitions from disk")
	log.Info().Int("count", len(instanceDefinitions.Agents)).Msg("loaded agent definitions from disk")

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
	players                []Player
	mapId                  int
	definition             instanceDefinition
	alive                  bool
	agents                 []Agent
	gracefulShutdownSignal chan bool
	forceShutdownSignal    chan bool
	log                    zerolog.Logger
}

func (inst *Instance) RemovePlayer(player *Player) {
	for i, v := range inst.players {
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
	for _, agentName := range i.definition.AgentNames {
		agentDefinition, ok := instanceDefinitions.Agents[agentName]
		if !ok {
			log.Error().Str("name", agentName).Msg("missing definition for agent")
		}
		ag := Agent{
			id:                 i.NextFreeAgentId(),
			posX:               float32(agentDefinition.SpawnPoint[0]),
			posY:               float32(agentDefinition.SpawnPoint[1]),
			plane:              agentDefinition.SpawnPoint[2],
			facingX:            1.0,
			facingY:            0.0,
			speed:              agentDefinition.Speed,
			modelId:            agentDefinition.ModelId,
			agentType:          agentDefinition.AgentType,
			modelType:          agentDefinition.ModelType,
			encName:            agentDefinition.EncName,
			profession:         agentDefinition.Profession,
			level:              agentDefinition.Level,
			fileId:             agentDefinition.FileId,
			unkPropertiesBytes: agentDefinition.UnkPropertiesBytes, // Really what is this? you can set to all 0 and it seems the same?
		}
		i.agents = append(i.agents, ag)
		log.Info().Str("name", agentName).Msg("added agent!")
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
				if player.client.closed {
					continue
				}
				player.EnqueuePacket(newPingRequest(30, 491)) // dont know what these values mean
			}
		}
	}
}

func (i *Instance) MovementTickLoop() {
	for i.alive {
		time.Sleep(time.Millisecond * 500)
		for _, player := range i.players {
			if player.client.closed {
				continue
			}
			player.EnqueuePacket(newAgentMovementTick(500))
		}
	}
}

func (i *Instance) NextFreeAgentId() int {
	return len(i.agents) + 1
}

func (i *Instance) NextSpawnPoint() (x, y float32, plane int) {
	nSpawnPoints := len(i.definition.SpawnPoints)
	if nSpawnPoints == 0 {
		panic(fmt.Errorf("instance for map id %d has no spawn points", i.mapId))
	}
	spawnPoint := i.definition.SpawnPoints[0]
	x = float32(spawnPoint[0])
	y = float32(spawnPoint[1])
	plane = spawnPoint[2]
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
		// Append high byte then low byte (big-endian)
		conv = append(conv, byte(val&0xff), byte(val>>8))
	}
	return conv
	//s := string([]byte{0x2d, 0x9e, 0xf8, 0x78, 0xbd, 0xbf, 0x12, 0xe7})
	/*converted, err := parseUTF16HexString(in)
	if err != nil {
		panic(err)
	}
	return converted*/
	//return s
}

func (i *Instance) AddPlayer(player *Player) {
	i.players = append(i.players, *player)
	player.agentId = i.NextFreeAgentId()
	i.log.Info().Int("agentId", player.agentId).Msg("assigned player agent id")
	i.log.Debug().Uint64("playerUuid", player.uuid).Msg("player added to instance")
	player.EnqueuePacket(newInstanceLoadHead())
	if i.IsCharCreationInstance() {
		player.EnqueuePacket(newCharCreationStart())
		player.client.sendCreateCharacterInstanceInfo()
	} else {
		player.posX, player.posY, player.plane = i.NextSpawnPoint()
		player.client.sendWorldInstanceHead()
		player.client.sendWorldInstanceBody()
		player.EnqueuePacket(newUpdateMapId(i.mapId))
		player.EnqueuePacket(newReadyForMapSpawn())
		player.EnqueuePacket(newInstanceManifestDone(0, 1, 0))
	}
}

func (i *Instance) SendActiveAgents(to *Player) {
	// Let's send agent info now.
	for _, ag := range i.agents {
		npcId := ag.agentType & 0xffff
		to.EnqueuePacket(newAgentUpdateNPCProperties(npcId, ag.fileId, ag.profession, ag.level, convertEncName(ag.unkPropertiesBytes)))
		to.EnqueuePacket(newAgentUpdateNPCModel(npcId, ag.modelId))
		to.EnqueuePacket(newAgentUpdateNPCName(ag.id, convertEncName(ag.encName)))
		to.EnqueuePacket(newAgentInitialEffects(ag.id, 0))
		to.EnqueuePacket(newAgentSpawned(ag.id, ag.agentType, 1, 9, ag.modelType, ag.posX, ag.posY, ag.plane, ag.facingX, ag.facingY, ag.speed))
	}
}
