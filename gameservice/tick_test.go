package gameservice

import (
	"math"
	"math/rand"
	"net"
	"testing"
	"time"

	GwPacket "gw1/server/gwpacket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventually polls cond until it returns true or the timeout elapses. Used to
// observe effects that are applied by the instance actor's game tick, which
// runs on its own goroutine.
func eventually(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

// newTickTestInstance returns an Instance with no actor goroutine, so tests can
// drive the tick phases synchronously without racing the real actor.
func newTickTestInstance() *Instance {
	i := &Instance{
		uuid:                   rand.Uint64(),
		tag:                    rand.Uint32(),
		alive:                  true,
		agents:                 make([]Agent, 0),
		gracefulShutdownSignal: make(chan bool, 1),
		forceShutdownSignal:    make(chan bool, 1),
		cmdCh:                  make(chan instanceMsg, 64),
		done:                   make(chan struct{}),
	}
	i.log = log.With().Uint64("uuid", i.uuid).Logger()
	return i
}

// hasPositionUpdate reports whether a sink packet is a MarshalAgentUpdatePosition
// (0x2c) carrying the given coordinates.
func hasPositionUpdate(packets [][]byte, wantX, wantY float32) bool {
	for _, p := range packets {
		if len(p) < 14 || int(p[0])|(int(p[1])<<8) != 0x2c {
			continue
		}
		x := math.Float32frombits(uint32(p[6]) | uint32(p[7])<<8 | uint32(p[8])<<16 | uint32(p[9])<<24)
		y := math.Float32frombits(uint32(p[10]) | uint32(p[11])<<8 | uint32(p[12])<<16 | uint32(p[13])<<24)
		if x == wantX && y == wantY {
			return true
		}
	}
	return false
}

// TestTick_Phase1CollectsPhase2Applies verifies the two-phase split directly:
// phase 1 (intent setter) does not touch world state, phase 2 applies it.
func TestTick_Phase1CollectsPhase2Applies(t *testing.T) {
	inst := newTickTestInstance()
	bot, botSink := newTestPlayer("Bot")
	watcher, watcherSink := newTestPlayer("Watcher")
	bot.playerId = 1
	bot.agentId = 2
	inst.players = append(inst.players, bot, watcher)

	require.NoError(t, bot.setMoveIntent(&MoveToPoint{x: 123.5, y: 456.25, plane: 0}))

	assert.Equal(t, float32(0), bot.posX, "phase 1 must not mutate world state")
	assert.NotNil(t, bot.pendingMove, "phase 1 records intent")

	inst.inActor.Store(true)
	disconnect := inst.applyPlayerIntents(bot)
	inst.inActor.Store(false)

	assert.False(t, disconnect)
	assert.Equal(t, float32(123.5), bot.posX)
	assert.Equal(t, float32(456.25), bot.posY)
	assert.Nil(t, bot.pendingMove, "intent is consumed in the same tick")
	assert.True(t, watcherSink.hasOpcode(0x2c), "move must be broadcast to watcher")
	assert.True(t, botSink.hasOpcode(0x2c), "move broadcast reaches the mover")
}

// TestTick_ChatAppliedSameTick verifies chat intents are broadcast in phase 2
// and cleared within the tick.
func TestTick_ChatAppliedSameTick(t *testing.T) {
	inst := newTickTestInstance()
	bot, _ := newTestPlayer("Bot")
	watcher, watcherSink := newTestPlayer("Watcher")
	bot.playerId = 1
	inst.players = append(inst.players, bot, watcher)

	require.NoError(t, bot.setChatIntent(&ChatMessage{agentId: 2, message: "!hello everyone"}))
	watcherSink.reset()

	inst.inActor.Store(true)
	disconnect := inst.applyPlayerIntents(bot)
	inst.inActor.Store(false)

	assert.False(t, disconnect)
	assert.Nil(t, bot.pendingChat, "chat intent consumed in the same tick")
	assert.True(t, watcherSink.hasOpcode(0x5c), "local chat must be broadcast")
}

// setupTickWorld wires a real GSConn (already handed over to an instance) plus
// a headless watcher into a live instance, and drains the client socket in the
// background so the actor's per-tick flush never blocks on a full kernel
// buffer.
func setupTickWorld(t *testing.T) (*Instance, *GSConn, *headlessSink) {
	t.Helper()
	clientConn, conn := setupTestConn(t)
	t.Cleanup(func() { conn.Close() })

	inst := newTestInstance(t)
	conn.state = StateVerified
	conn.handedOver.Store(true)
	require.NoError(t, inst.AddPlayer(conn.player))

	watcher, watcherSink := newTestPlayer("Watcher")
	require.NoError(t, inst.AddPlayer(watcher))
	watcherSink.reset()

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-stop:
				return
			default:
			}
			clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			if _, err := clientConn.Read(buf); err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				return
			}
		}
	}()

	return inst, conn, watcherSink
}

func moveToPointPacket(x, y float32) []byte {
	out := GwPacket.NewOut(0x803d)
	out.Float32(x)
	out.Float32(y)
	out.Uint32(0)
	return out.GetBytes()
}

// TestGameTick_DrainsBufferedMoveOnActorTick verifies the full pipeline:
// bytes buffered on the connection are drained by the instance actor's game
// tick, applied as intent, and broadcast — not processed on arrival.
func TestGameTick_DrainsBufferedMoveOnActorTick(t *testing.T) {
	_, conn, watcherSink := setupTickWorld(t)

	require.NoError(t, conn.AppendIn(moveToPointPacket(123.5, 456.25)))

	eventually(t, 2*time.Second, func() bool {
		return hasPositionUpdate(watcherSink.packetsSent(), 123.5, 456.25)
	})
}

// TestGameTick_PartialPacketResumesNextTick verifies a fragmented packet stays
// buffered (nothing applied) until the rest arrives on a later tick.
func TestGameTick_PartialPacketResumesNextTick(t *testing.T) {
	_, conn, watcherSink := setupTickWorld(t)

	full := moveToPointPacket(123.5, 456.25)
	require.NoError(t, conn.AppendIn(full[:6]))

	time.Sleep(150 * time.Millisecond) // several ticks with an incomplete packet
	assert.False(t, hasPositionUpdate(watcherSink.packetsSent(), 123.5, 456.25),
		"incomplete packet must not be applied")

	require.NoError(t, conn.AppendIn(full[6:]))
	eventually(t, 2*time.Second, func() bool {
		return hasPositionUpdate(watcherSink.packetsSent(), 123.5, 456.25)
	})
}

// TestGameTick_ChatFromBufferBroadcastOnTick verifies a chat packet buffered on
// the connection is broadcast by the actor's tick.
func TestGameTick_ChatFromBufferBroadcastOnTick(t *testing.T) {
	_, conn, watcherSink := setupTickWorld(t)

	out := GwPacket.NewOut(0x8063)
	out.Uint32(0) // agentId
	out.UTF16WithLengthPrefix("!hello tick")
	require.NoError(t, conn.AppendIn(out.GetBytes()))

	eventually(t, 2*time.Second, func() bool {
		return watcherSink.hasOpcode(0x5c)
	})
}

// TestGameTick_ClientDisconnectRemovesPlayer verifies an in-instance disconnect
// is handled on the actor without deadlocking (it must not deliver to itself).
func TestGameTick_ClientDisconnectRemovesPlayer(t *testing.T) {
	_, conn, watcherSink := setupTickWorld(t)

	disc := GwPacket.NewOut(0x8008)
	require.NoError(t, conn.AppendIn(disc.GetBytes()))

	eventually(t, 2*time.Second, func() bool {
		return conn.IsClosed() && watcherSink.hasOpcode(0x21) // despawn broadcast
	})
}
