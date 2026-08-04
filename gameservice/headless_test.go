package gameservice

import (
	"gw1/server/gwpacket"
	"gw1/server/pathing"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// headlessSink is a playerConn that records every packet a Player "sends", so
// tests can exercise Player methods without a real GSConn.
type headlessSink struct {
	mu      sync.Mutex
	packets [][]byte
	closed  atomic.Bool
}

func (s *headlessSink) EnqueuePacket(out gwpacket.Out) {
	packet := append([]byte(nil), out.GetBytes()...)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packets = append(s.packets, packet)
}

func (s *headlessSink) IsClosed() bool {
	return s.closed.Load()
}

func (s *headlessSink) Close() {
	s.closed.Store(true)
}

func (s *headlessSink) clientIP() string {
	return "127.0.0.1"
}

func (s *headlessSink) HandedOver() bool {
	return true
}

func (s *headlessSink) DrainInInstance() error {
	return nil
}

func (s *headlessSink) Flush() error {
	return nil
}

func (s *headlessSink) FlushAsync() {
}

func (s *headlessSink) packetsSent() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.packets))
	for i, p := range s.packets {
		out[i] = append([]byte(nil), p...)
	}
	return out
}

func (s *headlessSink) opcodes() []int {
	sent := s.packetsSent()
	opcodes := make([]int, 0, len(sent))
	for _, p := range sent {
		if len(p) < 2 {
			continue
		}
		opcodes = append(opcodes, int(p[0])|(int(p[1])<<8))
	}
	return opcodes
}

func (s *headlessSink) hasOpcode(op int) bool {
	for _, o := range s.opcodes() {
		if o == op {
			return true
		}
	}
	return false
}

func (s *headlessSink) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packets = nil
}

func newTestPlayer(name string) (*Player, *headlessSink) {
	sink := &headlessSink{}
	p := newPlayer(sink, zerolog.Nop())
	p.name = name
	p.level = 20
	p.primaryProfession = 1
	return p, sink
}

var testPathStore = func() *pathing.Store {
	store := pathing.NewStore()
	for _, id := range []uint32{0x340c6, 0x1b97d, 0x1bacb} {
		store.Set(id, &pathing.PathData{})
	}
	return store
}()

func ensureTestPathStore(t *testing.T) {
	t.Helper()
	if instancePathStore == nil {
		instancePathStore = testPathStore
	}
}

func newTestInstance(t *testing.T) *Instance {
	return newTestInstanceForMap(t, 3)
}

func newTestInstanceForMap(t *testing.T, mapId int) *Instance {
	t.Helper()
	ensureTestPathStore(t)
	definition, ok := instanceDefinitions.Instances[mapId]
	if !ok {
		t.Fatalf("missing test instance definition for map id %d", mapId)
	}
	inst, err := NewInstance(mapId, definition)
	if err != nil {
		t.Fatalf("failed to create instance for map id %d: %v", mapId, err)
	}
	t.Cleanup(inst.Shutdown)
	return inst
}

// newHeadlessInstance returns a fully-populated Instance with no actor goroutine,
// so tests can drive the instance methods synchronously.
func newHeadlessInstance(t *testing.T, mapId int) *Instance {
	t.Helper()
	ensureTestPathStore(t)
	definition, ok := instanceDefinitions.Instances[mapId]
	if !ok {
		t.Fatalf("missing test instance definition for map id %d", mapId)
	}
	inst, err := newInstance(mapId, definition)
	if err != nil {
		t.Fatalf("failed to create instance for map id %d: %v", mapId, err)
	}
	inst.inActor.Store(true)
	return inst
}

func TestAddPlayerAssignsIdsAndSetsConnectedInstance(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	player, sink := newTestPlayer("TestPlayer")

	inst.AddPlayer(player)

	assert.Equal(t, 1, player.agentId)
	assert.Equal(t, 1, player.playerId)
	assert.Equal(t, inst, player.connectedInstance)
	assert.Equal(t, 1, len(inst.players))

	assert.True(t, sink.hasOpcode(0x17b), "expected MarshalInstanceLoadHead")
}

func TestAddPlayerTransmitsPlayerToOthers(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	first, firstSink := newTestPlayer("First")
	second, _ := newTestPlayer("Second")

	inst.AddPlayer(first)
	firstSink.reset()
	inst.AddPlayer(second)

	assert.True(t, firstSink.hasOpcode(0x58), "expected MarshalAgentCreatePlayer for second player")
	assert.Equal(t, 2, len(inst.players))
}

func TestStartPlayerMoveBroadcastsMovement(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	bot, botSink := newTestPlayer("Bot")
	watcher, watcherSink := newTestPlayer("Watcher")

	inst.AddPlayer(bot)
	inst.AddPlayer(watcher)
	bot.posX, bot.posY, bot.plane = 0, 50, 0
	bot.baseSpeed = 288
	botSink.reset()
	watcherSink.reset()

	inst.startPlayerMove(bot, 123.5, 456.25, 0)

	assert.Equal(t, float32(123.5), bot.destX)
	assert.Equal(t, float32(456.25), bot.destY)
	assert.True(t, watcherSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in watcher sink")
	assert.True(t, botSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in moving player sink")
}

func TestRemovedPlayerNotAdvancedBySim(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	player, _ := newTestPlayer("Leaver")

	inst.AddPlayer(player)
	inst.RemovePlayer(player)
	player.posX, player.posY, player.plane = 0, 50, 0
	player.baseSpeed = 288
	player.waypoints = []pathing.Waypoint{{X: 0, Y: -50, Plane: 0, TrapID: 1}}
	player.waypointIdx = 1
	player.destX, player.destY, player.destPlane = 0, -50, 0
	inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)

	inst.tickMovement()

	assert.Equal(t, float32(50), player.posY)
}

func TestSendChatRecordsPacket(t *testing.T) {
	player, sink := newTestPlayer("Chatter")

	player.SendChat("hello", 3)
	player.SendChatWarning("careful")

	sent := sink.opcodes()
	assert.Len(t, sent, 2)
	assert.Contains(t, sent, 0x5c)
}

func TestRemovePlayerBroadcastsDespawn(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	bot, _ := newTestPlayer("Bot")
	watcher, watcherSink := newTestPlayer("Watcher")

	inst.AddPlayer(bot)
	inst.AddPlayer(watcher)
	watcherSink.reset()

	inst.RemovePlayer(bot)

	assert.Equal(t, 1, len(inst.players))
	assert.Equal(t, watcher.playerId, inst.players[0].playerId)
	assert.True(t, watcherSink.hasOpcode(0x21), "expected MarshalAgentDespawned for bot")
}

func TestBroadcastGenericReachesAllPlayers(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	first, firstSink := newTestPlayer("First")
	second, secondSink := newTestPlayer("Second")

	inst.AddPlayer(first)
	inst.AddPlayer(second)
	firstSink.reset()
	secondSink.reset()

	inst.BroadcastGeneric(gwpacket.NewOut(0x1234))

	assert.Contains(t, firstSink.opcodes(), 0x1234)
	assert.Contains(t, secondSink.opcodes(), 0x1234)
}

func TestBroadcastLocalChatReachesAllPlayers(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	first, firstSink := newTestPlayer("First")
	second, secondSink := newTestPlayer("Second")

	inst.AddPlayer(first)
	inst.AddPlayer(second)
	firstSink.reset()
	secondSink.reset()

	inst.BroadcastLocalChat(first, "hello everyone")

	assert.Contains(t, firstSink.opcodes(), 0x5c)
	assert.Contains(t, secondSink.opcodes(), 0x5c)
}

func TestSendActiveAgentsIncludesNPCsAndPlayers(t *testing.T) {
	inst := newHeadlessInstance(t, 165)
	player, playerSink := newTestPlayer("Player")
	other, _ := newTestPlayer("Other")

	inst.AddPlayer(player)
	inst.AddPlayer(other)
	playerSink.reset()

	inst.SendActiveAgents(player)

	ops := playerSink.opcodes()
	assert.Contains(t, ops, 0x9a, "expected MarshalAgentUpdateNPCName for NPC agents")
	assert.Contains(t, ops, 0x58, "expected MarshalAgentCreatePlayer for the other player")
	assert.Contains(t, ops, 0x20, "expected MarshalAgentSpawned")
}

func TestInstanceLoadRequestPlayersSendsSpawnSequence(t *testing.T) {
	inst := newHeadlessInstance(t, 165)
	player, sink := newTestPlayer("Player")

	inst.AddPlayer(player)
	player.itemMgr.AddBag(20, 1)
	player.itemMgr.AddBag(9, 2)
	sink.reset()

	player.sendInstanceLoadRequestPlayers(InstanceLoadRequestPlayers{})

	for _, op := range []int{0x58, 0x20, 0xaf, 0x1d1, 0x18d, 0x9a} {
		assert.Contains(t, sink.opcodes(), op, "expected spawn sequence opcode %04x", op)
	}
}

func TestCommandsRecordPackets(t *testing.T) {
	player, sink := newTestPlayer("Commander")

	assert.True(t, handleSpeedCommand(player, []string{"288"}))
	assert.True(t, sink.hasOpcode(0x27), "expected MarshalAgentUpdateSpeedBase")

	sink.reset()
	assert.True(t, handleColorCommand(player, nil))
	assert.Len(t, sink.opcodes(), 14)

	sink.reset()
	assert.True(t, handleMotdCommand(player, nil))
	assert.NotEmpty(t, sink.opcodes())

	sink.reset()
	player.posX, player.posY, player.plane = 123.4, 567.8, 2
	assert.True(t, handlePosCommand(player, nil))
	assert.True(t, sink.hasOpcode(0x5c), "expected MarshalChatMessageFromServer")
}

func TestDisconnectClosesSink(t *testing.T) {
	player, sink := newTestPlayer("Leaver")

	assert.False(t, sink.IsClosed())

	player.Disconnect()

	assert.True(t, sink.IsClosed())
}
