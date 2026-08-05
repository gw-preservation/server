package gameservice

import (
	"fmt"
	"gw1/server/db"
	"gw1/server/packet"
	"gw1/server/pathing"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"github.com/rs/zerolog"
)

var log zerolog.Logger
var ServerIP [4]byte

func init() {
	log = zerolog.New(zerolog.NewConsoleWriter())
	log = log.With().Timestamp().Logger()
}

type agentSpawnInfo struct {
	Name       string
	Level      int
	SpawnPoint [3]float32
}

type instanceDefinition struct {
	Name        string
	Expansion   string
	IsPvp       bool
	Explorable  bool
	MapFileId   int
	PartySize   int
	Agents      []agentSpawnInfo
	SpawnPoints [][]float32
}

type agentDefinition struct {
	Name               string
	EncName            string
	ModelId            int
	AllegianceFlags    int
	Speed              float32
	Profession         int
	FileId             int
	UnkPropertiesBytes string
	DefinitionIndex    int
}

// InitializeInstances assigns agent definition indices and prepares lazy
// pathing.
func InitializeInstances(gwdatPath string) error {
	index := 0
	for name := range instanceDefinitions.Agents {
		def := instanceDefinitions.Agents[name]
		def.DefinitionIndex = index
		instanceDefinitions.Agents[name] = def
		index++
	}
	return initializePathing(gwdatPath)
}

func GetMapIdForNameCaseInsensitive(name string) (int, bool) {
	name = strings.ToLower(name)
	for mapId, definition := range instanceDefinitions.Instances {
		if strings.ToLower(definition.Name) == name {
			return mapId, true
		}
	}
	return 0, false
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
	definition, ok := instanceDefinitions.Instances[mapId]
	if !ok {
		return nil, fmt.Errorf("missing instance definition for map id %d", mapId)
	}
	existingInst, hasExistingInst := im.GetInstanceByMapId(mapId)
	if hasExistingInst {
		return existingInst, nil
	}
	inst, err := NewInstance(mapId, definition)
	if err != nil {
		return nil, err
	}
	im.AddInstance(inst)
	return inst, nil
}

func (im *instanceManager) BroadcastPacketToAllPlayers(packet packet.Out) {
	im.mu.Lock()
	defer im.mu.Unlock()
	for _, inst := range im.instances {
		inst.BroadcastGeneric(packet)
	}
}

func (im *instanceManager) NumPlayersOnline() int {
	im.mu.RLock()
	defer im.mu.RUnlock()
	x := 0
	for _, inst := range im.instances {
		x += int(inst.playerCount.Load())
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
}

func (im *instanceManager) RemoveInstance(uuid uint64) {
	im.mu.Lock()
	delete(im.instances, uuid)
	im.mu.Unlock()
}

// Instance is single-writer: every field is owned by the actor goroutine and
// unsynchronized. Never touch instance state off the actor.
type Instance struct {
	uuid                   uint64
	tag                    uint32
	players                []*Player
	mapId                  int
	definition             instanceDefinition
	path                   *pathing.PathData // navmesh for this map, shared read-only; never nil (NewInstance fails without one)
	transitions            []MapPortalDefinition
	alive                  bool
	agents                 []Agent
	gracefulShutdownSignal chan bool
	forceShutdownSignal    chan bool
	log                    zerolog.Logger

	pendingJoins          chan *Player
	done                  chan struct{}
	closeOnce             sync.Once
	inActor               atomic.Bool
	playerCount           atomic.Int64
	lastTickAt            time.Time
	lastMovementAdvanceAt time.Time
	tickCount             int64
}

func (i *Instance) assertActor() {
	if !i.inActor.Load() {
		panic("assertActor: called outside actor goroutine")
	}
}

func (i *Instance) AcceptPlayer(p *Player) {
	select {
	case i.pendingJoins <- p:
	case <-i.done:
	}
}

func (inst *Instance) TransmitAgentDespawned(agent *Agent) {
	inst.assertActor()
	for _, other := range inst.players {
		other.sendAgentDespawned(agent)
	}
}

func (inst *Instance) RemovePlayer(player *Player) {
	inst.assertActor()
	removed := false
	for idx, v := range inst.players {
		if v == nil {
			continue
		}
		if player.uuid == v.uuid {
			inst.players = slices.Delete(inst.players, idx, idx+1)
			removed = true
			break
		}
	}
	inst.playerCount.Store(int64(len(inst.players)))
	if removed {
		inst.TransmitAgentDespawned(&player.Agent)
		inst.log.Debug().Uint64("playerUuid", player.uuid).Msg("player removed from instance")
		if len(inst.players) == 0 {
			inst.log.Debug().Msg("instance shutting down due to inactivity")
			inst.gracefulShutdownSignal <- true
		}
	}
}

// NewInstance builds an Instance for the given map, refusing to create one
// without a navmesh.
func NewInstance(mapId int, definition instanceDefinition) (*Instance, error) {
	i, err := newInstance(mapId, definition)
	if err != nil {
		return nil, err
	}
	go i.actorLoop()
	return i, nil
}

func newInstance(mapId int, definition instanceDefinition) (*Instance, error) {
	sd, err := ensurePathLoaded(uint32(definition.MapFileId), definition.Name)
	if err != nil {
		return nil, err
	}
	i := &Instance{
		definition:             definition,
		uuid:                   rand.Uint64(),
		tag:                    rand.Uint32(),
		mapId:                  mapId,
		alive:                  true,
		path:                   sd,
		agents:                 make([]Agent, 0),
		gracefulShutdownSignal: make(chan bool, 1),
		forceShutdownSignal:    make(chan bool, 1),
		pendingJoins:           make(chan *Player, 16),
		done:                   make(chan struct{}),
	}
	i.transitions = mapTransitions[mapId]
	i.log = log.With().Uint64("uuid", i.uuid).Int("mapId", i.mapId).Logger()
	if i.definition.Explorable {
		i.log.Debug().Msg("created a new explorable instance")
	}

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
			baseSpeed:           agentDefinition.Speed,
			modelId:             agentDefinition.ModelId,
			allegianceFlags:     agentDefinition.AllegianceFlags,
			encName:             agentDefinition.EncName,
			primaryProfession:   agentDefinition.Profession,
			secondaryProfession: 0,
			level:               agentToSpawn.Level,
			fileId:              agentDefinition.FileId,
			unkPropertiesBytes:  agentDefinition.UnkPropertiesBytes,
		}
		i.agents = append(i.agents, ag)
	}
	return i, nil
}

const targetGameTick = 50 * time.Millisecond

func (i *Instance) actorLoop() {
	ping := time.NewTicker(5 * time.Second)
	tickTimer := time.NewTimer(targetGameTick)
	defer ping.Stop()
	defer tickTimer.Stop()
	defer func() {
		i.alive = false
		i.finish()
		InstanceManager.RemoveInstance(i.uuid)
	}()
	for {
		select {
		case <-ping.C:
			i.inActor.Store(true)
			i.pingPlayers()
			i.flushPlayers()
			i.inActor.Store(false)
		case <-tickTimer.C:
			tickStart := time.Now()
			i.gameTick()
			elapsed := time.Since(tickStart)
			if elapsed > 50*time.Millisecond {
				i.log.Warn().Dur("elapsed", elapsed).Msg("slow game tick")
			}
			delay := targetGameTick - elapsed
			if delay < 0 {
				delay = 0
			}
			tickTimer.Reset(delay)
		case <-i.gracefulShutdownSignal:
			i.log.Debug().Msg("graceful shutdown")
			return
		case <-i.forceShutdownSignal:
			i.log.Debug().Msg("force shutdown")
			return
		}
	}
}

func (i *Instance) gameTick() {
	i.inActor.Store(true)
	defer i.inActor.Store(false)

	// Phase 0: admit pending joins.
drainJoins:
	for {
		select {
		case p := <-i.pendingJoins:
			if !p.conn.IsClosed() {
				i.AddPlayer(p)
			}
		default:
			break drainJoins
		}
	}

	// Phase 1: drain each player's buffered input (handlers will record intent).
	for _, p := range i.players {
		if !p.conn.HandedOver() {
			continue
		}
		if err := p.conn.DrainInInstance(); err != nil {
			i.log.Error().Err(err).Uint64("playerUuid", p.uuid).Msg("drain failed, disconnecting player")
			p.pendingDisconnect = true
		}
	}

	// Phase 2: world logic based on player intent (movement, interaction, etc).
	var disconnect []*Player
	for _, p := range i.players {
		if p.conn.IsClosed() {
			disconnect = append(disconnect, p)
			continue
		}
		if !i.processPlayer(p) {
			disconnect = append(disconnect, p)
		}
	}

	i.tickMovement()

	// Phase 3: flush outbound, then disconnect any players that were marked for disconnect.
	i.flushPlayers()
	for _, p := range disconnect {
		i.RemovePlayer(p)
		p.OnUserDisconnected()
		p.conn.Close()
	}

	i.lastTickAt = time.Now()
	i.tickCount++
}

func (i *Instance) processPlayer(p *Player) bool {
	i.assertActor()
	if p.pendingDisconnect {
		p.pendingDisconnect = false
		return false
	}

	if m := p.moveTo; m != nil {
		p.moveTo = nil
		i.startPlayerMove(p, m.x, m.y, m.plane)
	}

	if p.cancelInteractRequested {
		p.cancelInteractRequested = false
	}

	if it := p.interact; it != nil {
		p.interact = nil
		p.SendChatWarning(fmt.Sprintf("missing interaction definition for agent=%d,action=%d", it.agentId, it.action))
		p.log.Debug().Int("target", it.agentId).Int("action", it.action).Msg("InteractAgent")
	}

	if t := p.target; t != nil {
		p.target = nil
		p.log.Debug().Int("target", t.targetAgentId).Str("playerName", p.name).Msg("UpdateTarget")
	}

	if e := p.equip; e != nil {
		p.equip = nil
		p.log.Info().Int("itemLocalId", e.itemLocalId).Msg("EquipItem")
		p.TryEquipItem(e.itemLocalId)
	}

	if d := p.destroy; d != nil {
		p.destroy = nil
		p.log.Info().Int("itemLocalId", d.itemLocalId).Msg("DestroyItem")
		if err := p.itemMgr.RemoveItemByLocalId(d.itemLocalId); err != nil {
			p.log.Error().Err(err).Int("itemLocalId", d.itemLocalId).Msg("DestroyItem")
		}
	}

	if p.loadSpawnRequested {
		p.loadSpawnRequested = false
		p.sendInstanceLoadSpawnPoint()
	}

	if payload := p.loadPlayers; payload != nil {
		p.loadPlayers = nil
		p.sendInstanceLoadRequestPlayers(*payload)
	}

	if m := p.mapTravel; m != nil {
		p.mapTravel = nil
		i.log.Info().Int("mapId", m.mapId).Msg("MapTravel")
		if err := i.TransferPlayerToNewMap(p, m.mapId, 0, 0, -1); err != nil {
			p.log.Error().Err(err).Int("mapId", m.mapId).Msg("MapTravel")
		}
	}

	if c := p.chat; c != nil {
		p.chat = nil
		i.applyChat(p, c)
	}

	return true
}

func (i *Instance) applyChat(p *Player, c *ChatMessage) {
	i.assertActor()
	msg := c.message
	if len(msg) <= 1 {
		return
	}
	p.log.Info().Int("ag", c.agentId).Str("msg", msg).Msg("ChatMessage")

	channel := msg[0]
	remainder := msg[1:]
	switch channel {
	case '!':
		i.BroadcastLocalChat(p, remainder)
	case '/':
		words := strings.Fields(remainder)
		if len(words) == 0 {
			p.SendChatWarning("Invalid command syntax")
			return
		}
		command := words[0]
		args := words[1:]
		if emote, exists := GetEmoteByCommand(command); exists {
			i.BroadcastGeneric(MarshalEmote(p.playerId, p.agentId, emote))
			return
		}
		HandleCommand(p, command, remainder, args)
	}
}

func (i *Instance) flushPlayers() {
	i.assertActor()
	for _, p := range i.players {
		if p.conn.IsClosed() {
			continue
		}
		// don't stall tick cycle if flush takes some time, call async.
		p.conn.FlushAsync()
	}
}

func (i *Instance) pingPlayers() {
	i.assertActor()
	for _, player := range i.players {
		if player.conn.IsClosed() {
			continue
		}
		player.EnqueuePacket(MarshalServerPingRequest(30, 100))
	}
}

func (i *Instance) finish() {
	i.closeOnce.Do(func() {
		close(i.done)
	})
}

func (i *Instance) Shutdown() {
	select {
	case i.forceShutdownSignal <- true:
	default:
	}
	i.finish()
}

func randomFloatAround(start, rangeVal float32) float32 {
	offset := (rand.Float32() * 2 * rangeVal) - rangeVal
	return start + offset
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func (i *Instance) NextFreeAgentId() int {
	return len(i.agents) + 1
}
func (i *Instance) NextFreePlayerId() int {
	return len(i.players) + 1
}

func (i *Instance) NextSpawnPoint() (x, y float32, plane int) {
	nSpawnPoints := len(i.definition.SpawnPoints)
	if nSpawnPoints == 0 {
		x = 0.0
		y = 0.0
		plane = 0
		return
	}
	randIndex := rand.Intn(nSpawnPoints)
	spawnPoint := i.definition.SpawnPoints[randIndex]
	x = randomFloatAround(spawnPoint[0], 100.0)
	y = randomFloatAround(spawnPoint[1], 100.0)
	plane = int(spawnPoint[2])
	return
}

func parseUTF16HexString(s string) (string, error) {
	parts := strings.Fields(s)
	var codeUnits []uint16
	for _, part := range parts {
		val, err := strconv.ParseUint(part, 16, 16)
		if err != nil {
			return "", fmt.Errorf("invalid hex code unit %q: %w", part, err)
		}
		codeUnits = append(codeUnits, uint16(val))
	}
	runes := utf16.Decode(codeUnits)
	return string(runes), nil
}

func convertEncName(in string) []byte {
	conv := []byte{}
	fields := strings.Fields(in)
	for _, word := range fields {
		val, err := strconv.ParseUint(word, 16, 16)
		if err != nil {
			panic(fmt.Errorf("invalid hex word %q: %w", word, err))
		}
		conv = append(conv, byte(val&0xff), byte(val>>8))
	}
	return conv
}

func (i *Instance) AddPlayer(player *Player) {
	i.assertActor()
	player.agentId = i.NextFreeAgentId()
	player.playerId = i.NextFreePlayerId()
	player.connectedInstance = i
	i.players = append(i.players, player)
	i.agents = append(i.agents, player.Agent)
	i.playerCount.Store(int64(len(i.players)))
	i.log.Info().Int("count", len(i.players)).Msgf("%s added to instance", player.name)
	for idx, v := range i.players {
		i.log.Debug().Int("index", idx).Int("playerID", v.playerId).Int("agentID", v.agentId).Str("name", v.name).Msg("player in instance")
	}
	player.EnqueuePacket(MarshalInstanceLoadHead())
	if player.hasPendingSpawn {
		spawnX := randomFloatAround(player.pendingSpawnX, 100.0)
		spawnY := randomFloatAround(player.pendingSpawnY, 100.0)
		player.posX, player.posY, player.plane = spawnX, spawnY, player.pendingSpawnPlane
		player.hasPendingSpawn = false
	} else {
		player.posX, player.posY, player.plane = i.NextSpawnPoint()
	}
	player.sendWorldInstanceHead()

	i.TransmitPlayerToOthers(player)
}

func (i *Instance) SendActiveAgents(to *Player) {
	i.assertActor()

	transmittedDefinitions := make([]int, 0)
	for _, ag := range i.agents {
		if ag.isPlayer {
			continue
		}

		if !contains(transmittedDefinitions, ag.definitionIndex) {
			to.EnqueuePacket(MarshalAgentUpdateNPCProperties(ag.definitionIndex, ag.fileId, ag.primaryProfession, ag.level, convertEncName(ag.unkPropertiesBytes)))
			to.EnqueuePacket(MarshalAgentUpdateNPCModel(ag.definitionIndex, ag.modelId))
			transmittedDefinitions = append(transmittedDefinitions, ag.definitionIndex)
		}

		to.EnqueuePacket(MarshalAgentUpdateNPCName(ag.agentId, convertEncName(ag.encName)))
		to.EnqueuePacket(MarshalAgentInitialEffects(ag.agentId, 0))
		agentType := (0x2000 << 16) | ag.definitionIndex
		to.EnqueuePacket(MarshalAgentSpawned(
			ag.agentId,
			agentType,
			1,
			9,
			ag.posX, ag.posY, ag.plane,
			ag.facingX, ag.facingY,
			ag.baseSpeed,
			ag.allegianceFlags,
		))
		i.log.Info().Int("agentId", ag.agentId).Int("ToAgId", to.agentId).Int("ToPlayerId", to.playerId).Msg("Transmitted Agent")
	}
	i.TransmitOtherPlayersToPlayer(to)
}

func (i *Instance) TransmitOtherPlayersToPlayer(to *Player) {
	i.assertActor()
	for _, other := range i.players {
		if other.playerId == to.playerId {
			continue
		}
		i.TransmitPlayer(to, other)
	}
}

func (i *Instance) TransmitPlayerToOthers(player *Player) {
	i.assertActor()
	for _, other := range i.players {
		if other.playerId == player.playerId {
			continue
		}
		i.TransmitPlayer(other, player)
	}
}

func (i *Instance) TransmitPlayer(to *Player, other *Player) {
	i.assertActor()
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
		other.baseSpeed,
		other.allegianceFlags,
	))
	to.EnqueuePacket(MarshalAgentAttrUpdateInt(30, other.agentId, other.playerId))
}

func (i *Instance) broadcastPlayerPos(player *Player) {
	for _, other := range i.players {
		other.EnqueuePacket(MarshalAgentUpdatePosition(player.agentId, player.posX, player.posY, player.plane))
	}
}

func (i *Instance) BroadcastGeneric(packet packet.Out) {
	i.assertActor()

	for _, other := range i.players {
		other.EnqueuePacket(packet)
	}
}

func (i *Instance) BroadcastLocalChat(from *Player, message string) {
	i.assertActor()
	packet := MarshalChatMessageCore(fmt.Sprintf("\u0108\u0107%s\u0001", message))
	packet.Merge(MarshalChatMessageLocal(from.playerId, 3))
	i.BroadcastGeneric(packet)
}

func (i *Instance) GetTag() uint32 {
	return i.tag
}

func (i *Instance) TransferPlayerToNewMap(player *Player, newMapId int, spawnX, spawnY float32, spawnPlane int) error {
	i.assertActor()
	inst, err := InstanceManager.GetOrCreateInstanceByMapId(newMapId)
	if inst == nil || err != nil {
		player.log.Error().Err(err).Msg("unable to create instance")
		player.Disconnect()
		return nil
	}

	instanceTag := inst.GetTag()
	securityTag := GenerateConnectionTokenForInstance(instanceTag, true, player.dbChar.UUID, player.dbAcc.UUID, player.conn.clientIP())
	if spawnPlane >= 0 {
		SetConnectionTokenSpawnPoint(securityTag, spawnX, spawnY, spawnPlane)
	}

	// Send transfer packets to client.
	region := -2
	player.conn.EnqueuePacket(MarshalTransferGameServerInfo([]byte{
		0x02, 0x00,
		0x17, 0xe0,
		ServerIP[0], ServerIP[1], ServerIP[2], ServerIP[3],
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, int(instanceTag), region, newMapId, i.IsExplorable(), int(securityTag)))
	player.conn.EnqueuePacket(MarshalUpdateCurrentMapId(newMapId))

	// Flush othe client before closing the connection.
	if err := player.conn.Flush(); err != nil {
		player.log.Error().Err(err).Msg("unable to flush transfer packets")
	}

	// Close the connection. The next game tick removes the player from this
	// instance (IsClosed check). The client then connects to the new instance.
	player.conn.Close()

	if err := db.SaveCharacterMapTransfer(player.dbChar.ID, uint16(newMapId), player.itemMgr.BuildDBBags(), !inst.IsExplorable()); err != nil {
		player.log.Error().Err(err).Msg("unable to save character map transfer data")
		return err
	}
	player.log.Info().Msg("Switched instances and synced db")
	return nil
}

func (i *Instance) IsExplorable() bool {
	return i.definition.Explorable
}
