package gameservice

import (
	"errors"
	"fmt"

	GwPacket "gw1/server/gwpacket"
)

// ErrInstanceShutdown is returned when an operation is submitted to an
// instance whose actor goroutine has already shut down.
var ErrInstanceShutdown = errors.New("gameservice: instance shutting down")

// instanceMsg is the wire format into the instance actor goroutine. Each
// message is pure data; the actor decides how to interpret it.
type instanceMsg interface {
	isInstanceMsg()
	reply() *msgResult
}

// msgResult is embedded by every message so callers can block on completion.
// The actor writes err and closes done; the caller observes the write after
// the channel close (happens-before).
type msgResult struct {
	done chan struct{}
	err  error
}

type msgBase struct {
	res msgResult
}

func (m *msgBase) isInstanceMsg()    {}
func (m *msgBase) reply() *msgResult { return &m.res }

type msgAddPlayer struct {
	msgBase
	player *Player
}

type msgRemovePlayer struct {
	msgBase
	player *Player
}

type msgTransferPlayer struct {
	msgBase
	player   *Player
	newMapId int
}

type msgMovePlayer struct {
	msgBase
	player *Player
	x, y   float32
}

type msgBroadcast struct {
	msgBase
	packet GwPacket.Out
}

type msgSendActiveAgents struct {
	msgBase
	to *Player
}

type msgLoadSpawnPoint struct {
	msgBase
	player *Player
}

type msgLoadRequestPlayers struct {
	msgBase
	player  *Player
	payload InstanceLoadRequestPlayers
}

// deliver submits a synchronous command to the instance actor and blocks
// until it has been executed (or the instance has shut down).
func (i *Instance) deliver(m instanceMsg) error {
	r := m.reply()
	r.done = make(chan struct{})
	select {
	case i.cmdCh <- m:
	case <-i.done:
		return ErrInstanceShutdown
	}
	select {
	case <-r.done:
		return r.err
	case <-i.done:
		return ErrInstanceShutdown
	}
}

func (i *Instance) AddPlayer(p *Player) error {
	return i.deliver(&msgAddPlayer{player: p})
}

func (i *Instance) RemovePlayer(p *Player) error {
	return i.deliver(&msgRemovePlayer{player: p})
}

func (i *Instance) TransferPlayerToNewMap(p *Player, newMapId int) error {
	return i.deliver(&msgTransferPlayer{player: p, newMapId: newMapId})
}

func (i *Instance) UpdateRequestedPlayerPos(p *Player, x, y float32) error {
	return i.deliver(&msgMovePlayer{player: p, x: x, y: y})
}

func (i *Instance) BroadcastGeneric(packet GwPacket.Out) error {
	return i.deliver(&msgBroadcast{packet: packet})
}

func (i *Instance) BroadcastLocalChat(from *Player, message string) error {
	packet := MarshalChatMessageCore(fmt.Sprintf("\u0108\u0107%s\u0001", message))
	packet.Merge(MarshalChatMessageLocal(from.playerId, 3))
	return i.BroadcastGeneric(packet)
}

func (i *Instance) SendActiveAgents(to *Player) error {
	return i.deliver(&msgSendActiveAgents{to: to})
}

func (i *Instance) LoadSpawnPoint(p *Player) error {
	return i.deliver(&msgLoadSpawnPoint{player: p})
}

func (i *Instance) LoadRequestPlayers(p *Player, payload InstanceLoadRequestPlayers) error {
	return i.deliver(&msgLoadRequestPlayers{player: p, payload: payload})
}

// apply executes a single message on the actor goroutine.
func (i *Instance) apply(m instanceMsg) {
	switch msg := m.(type) {
	case *msgAddPlayer:
		msg.res.err = i.addPlayer(msg.player)
		close(msg.res.done)
	case *msgRemovePlayer:
		msg.res.err = i.removePlayer(msg.player)
		close(msg.res.done)
	case *msgTransferPlayer:
		msg.res.err = i.transferPlayerToNewMap(msg.player, msg.newMapId)
		close(msg.res.done)
	case *msgMovePlayer:
		msg.res.err = i.updateRequestedPlayerPos(msg.player, msg.x, msg.y)
		close(msg.res.done)
	case *msgBroadcast:
		msg.res.err = i.broadcastGeneric(msg.packet)
		close(msg.res.done)
	case *msgSendActiveAgents:
		msg.res.err = i.sendActiveAgents(msg.to)
		close(msg.res.done)
	case *msgLoadSpawnPoint:
		msg.res.err = i.loadSpawnPoint(msg.player)
		close(msg.res.done)
	case *msgLoadRequestPlayers:
		msg.res.err = i.loadRequestPlayers(msg.player, msg.payload)
		close(msg.res.done)
	default:
		panic(fmt.Sprintf("gameservice: unknown instance message %T", m))
	}
}
