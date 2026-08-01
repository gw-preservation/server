package gameservice

import (
	GwPacket "gw1/server/gwpacket"
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

func (s *headlessSink) EnqueuePacket(out GwPacket.Out) {
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

func newTestInstance(t *testing.T) *Instance {
	return newTestInstanceForMap(t, 3)
}

func newTestInstanceForMap(t *testing.T, mapId int) *Instance {
	t.Helper()
	definition, ok := instanceDefinitions.Instances[mapId]
	if !ok {
		t.Fatalf("missing test instance definition for map id %d", mapId)
	}
	inst := NewInstance(mapId, definition)
	t.Cleanup(inst.Shutdown)
	return inst
}

// newHeadlessInstance returns a fully-populated Instance (real definition, NPC
// agents spawned) with no actor goroutine, so tests can drive the actor phases
// synchronously via runInActor without racing a live game tick.
func newHeadlessInstance(t *testing.T, mapId int) *Instance {
	t.Helper()
	definition, ok := instanceDefinitions.Instances[mapId]
	if !ok {
		t.Fatalf("missing test instance definition for map id %d", mapId)
	}
	return newInstance(mapId, definition)
}

// runInActor runs fn as if on the instance actor goroutine, satisfying the
// assertActor guard so tests can call the actor-only impl methods directly.
func runInActor(inst *Instance, fn func()) {
	inst.inActor.Store(true)
	defer inst.inActor.Store(false)
	fn()
}

func TestAddPlayerAssignsIdsAndSetsConnectedInstance(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	player, sink := newTestPlayer("TestPlayer")

	runInActor(inst, func() { inst.addPlayer(player) })

	assert.Equal(t, 1, player.agentId)
	assert.Equal(t, 1, player.playerId)
	assert.Same(t, inst, player.connectedInstance.Load())
	assert.Equal(t, 1, len(inst.players))

	// The load sequence is enqueued to the player's own sink.
	assert.True(t, sink.hasOpcode(0x17b), "expected MarshalInstanceLoadHead")
	assert.True(t, sink.hasOpcode(0x1aa), "expected MarshalReadyForMapSpawn")
	assert.True(t, sink.hasOpcode(0x196), "expected MarshalInstanceManifestDone")
	assert.True(t, sink.hasOpcode(0x98), "expected MarshalUpdateCurrentMapId")
}

func TestAddPlayerTransmitsPlayerToOthers(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	first, firstSink := newTestPlayer("First")
	second, _ := newTestPlayer("Second")

	runInActor(inst, func() {
		inst.addPlayer(first)
		firstSink.reset()
		inst.addPlayer(second)
	})

	// The first player should have received the second player's create/spawn packets.
	assert.True(t, firstSink.hasOpcode(0x58), "expected MarshalAgentCreatePlayer for second player")
	assert.Equal(t, 2, len(inst.players))
}

func TestUpdateRequestedPlayerPosBroadcastsMovement(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	bot, botSink := newTestPlayer("Bot")
	watcher, watcherSink := newTestPlayer("Watcher")

	runInActor(inst, func() {
		inst.addPlayer(bot)
		inst.addPlayer(watcher)
	})
	botSink.reset()
	watcherSink.reset()

	runInActor(inst, func() { inst.updateRequestedPlayerPos(bot, 123.5, 456.25) })

	assert.Equal(t, float32(123.5), bot.posX)
	assert.Equal(t, float32(456.25), bot.posY)
	assert.True(t, watcherSink.hasOpcode(0x2c), "expected MarshalAgentUpdatePosition in watcher sink")
	assert.True(t, botSink.hasOpcode(0x2c), "expected MarshalAgentUpdatePosition in moving player sink")
}

func TestUpdateRequestedPlayerPosRejectsRemovedPlayer(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	player, _ := newTestPlayer("Leaver")

	runInActor(inst, func() {
		inst.addPlayer(player)
		inst.removePlayer(player)
	})

	runInActor(inst, func() { inst.updateRequestedPlayerPos(player, 123.5, 456.25) })

	assert.Equal(t, float32(0), player.posX)
	assert.Equal(t, float32(0), player.posY)
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

	runInActor(inst, func() {
		inst.addPlayer(bot)
		inst.addPlayer(watcher)
	})
	watcherSink.reset()

	runInActor(inst, func() { inst.removePlayer(bot) })

	assert.Equal(t, 1, len(inst.players))
	assert.Equal(t, watcher.playerId, inst.players[0].playerId)
	assert.True(t, watcherSink.hasOpcode(0x21), "expected MarshalAgentDespawned for bot")
}

func TestBroadcastGenericReachesAllPlayers(t *testing.T) {
	inst := newHeadlessInstance(t, 3)
	first, firstSink := newTestPlayer("First")
	second, secondSink := newTestPlayer("Second")

	runInActor(inst, func() {
		inst.addPlayer(first)
		inst.addPlayer(second)
	})
	firstSink.reset()
	secondSink.reset()

	runInActor(inst, func() { inst.broadcastGeneric(GwPacket.NewOut(0x1234)) })

	assert.Contains(t, firstSink.opcodes(), 0x1234)
	assert.Contains(t, secondSink.opcodes(), 0x1234)
}

func TestSendActiveAgentsIncludesNPCsAndPlayers(t *testing.T) {
	inst := newHeadlessInstance(t, 165)
	player, playerSink := newTestPlayer("Player")
	other, _ := newTestPlayer("Other")

	runInActor(inst, func() {
		inst.addPlayer(player)
		inst.addPlayer(other)
	})
	playerSink.reset()

	runInActor(inst, func() { inst.sendActiveAgents(player) })

	ops := playerSink.opcodes()
	assert.Contains(t, ops, 0x9a, "expected MarshalAgentUpdateNPCName for NPC agents")
	assert.Contains(t, ops, 0x58, "expected MarshalAgentCreatePlayer for the other player")
	assert.Contains(t, ops, 0x20, "expected MarshalAgentSpawned")
}

func TestInstanceLoadRequestPlayersSendsSpawnSequence(t *testing.T) {
	inst := newHeadlessInstance(t, 165)
	player, sink := newTestPlayer("Player")

	runInActor(inst, func() { inst.addPlayer(player) })
	player.itemMgr.AddBag(20, 1)
	player.itemMgr.AddBag(9, 2)
	sink.reset()

	runInActor(inst, func() { inst.loadRequestPlayers(player, InstanceLoadRequestPlayers{}) })

	for _, op := range []int{0x58, 0x20, 0xaf, 0x1d1, 0x18d, 0x9a} {
		assert.Contains(t, sink.opcodes(), op, "expected spawn sequence opcode %04x", op)
	}
}

func TestCommandsRecordPackets(t *testing.T) {
	player, sink := newTestPlayer("Commander")

	assert.True(t, handleSpeedCommand(nil, player, []string{"288"}))
	assert.True(t, sink.hasOpcode(0x27), "expected MarshalAgentUpdateSpeedBase")

	sink.reset()
	assert.True(t, handleColorCommand(nil, player, nil))
	assert.Len(t, sink.opcodes(), 14)

	sink.reset()
	assert.True(t, handleMotdCommand(nil, player, nil))
	assert.NotEmpty(t, sink.opcodes())
}

func TestDisconnectClosesSink(t *testing.T) {
	player, sink := newTestPlayer("Leaver")

	assert.False(t, sink.IsClosed())

	player.Disconnect()

	assert.True(t, sink.IsClosed())
}

// TestConcurrentJoinsSerialize exercises the fire-and-forget join handoff: many
// connection goroutines call AcceptPlayer on the same live instance at once.
// The actor drains the join queue on its game tick and must admit every player
// with no data races. Run with -race.
func TestConcurrentJoinsSerialize(t *testing.T) {
	inst := newTestInstance(t)

	const n = 100
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, _ := newTestPlayer("Joiner")
			inst.AcceptPlayer(p)
		}()
	}
	wg.Wait()

	eventually(t, 2*time.Second, func() bool {
		return inst.playerCount.Load() == n
	})
}
