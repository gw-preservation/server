package GameService

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func init() {
	log = zerolog.New(zerolog.NewConsoleWriter())
	log = log.Level(zerolog.DebugLevel)
	log = log.With().Timestamp().Logger()
}

type instanceDefinition struct {
	DebugName   string  `json:"debug_name"`
	Explorable  bool    `json:"explorable"`
	MapFileId   int     `json:"map_file_id"`
	PartySize   int     `json:"party_size"`
	SpawnPoints [][]int `json:"spawn_points,omitempty"`
}

var instanceDefinitions = struct {
	Definitions map[string]instanceDefinition
}{Definitions: make(map[string]instanceDefinition, 0)}

func LoadInstanceDefinitionsFromDisk() {
	file, err := os.Open("gameservice/instance_definitions.json")
	if err != nil {
		panic(fmt.Sprintf("failed to load instance definitions: %v", err))
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&instanceDefinitions); err != nil {
		panic(fmt.Sprintf("failed to load instance definitions: %v", err))
	}
	log.Info().Int("count", len(instanceDefinitions.Definitions)).Msg("loaded instance definitions from disk")

	// Now start up all persistent instances:
	for mapIdStr, definition := range instanceDefinitions.Definitions {
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
	definition, ok := instanceDefinitions.Definitions[strconv.Itoa(mapId)]
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

func (i *Instance) AddPlayer(player *Player) {
	i.players = append(i.players, *player)
	player.agentId = i.NextFreeAgentId()
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
