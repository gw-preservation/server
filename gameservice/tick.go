package gameservice

import (
	"fmt"
	"strings"
)

// gameTick is the actor's fixed-timestep world update. It implements the
// tick-batched model, mirroring engine-ts's processClientsIn/processPlayers:
//
//	phase 1 (collect):  drain each player's buffered input; handlers record
//	                    the client's requests on the player (no world state)
//	phase 2 (apply):    processPlayer consumes the requests and mutates state
//	phase 3 (output):   movement-tick broadcast (folded into the game tick)
//
// Outbound buffers are flushed by the caller (actorLoop) after gameTick
// returns. Requests are consumed within this tick and never queue across
// ticks.
func (i *Instance) gameTick() {
	// Phase 1: collect requests. Handlers set fields on the player only, so a
	// flood of packets cannot mutate world state mid-tick.
	var disconnect []*Player
	for _, p := range i.players {
		if !p.conn.HandedOver() {
			continue
		}
		if err := p.conn.DrainInInstance(); err != nil {
			i.log.Error().Err(err).Uint64("playerUuid", p.uuid).Msg("drain failed, disconnecting player")
			p.pendingDisconnect = true
		}
	}

	// Phase 2: apply requests.
	for _, p := range i.players {
		if i.processPlayer(p) {
			disconnect = append(disconnect, p)
		}
	}

	// Phase 3: movement-tick broadcast.
	for _, p := range i.players {
		if p.conn.IsClosed() {
			continue
		}
		p.EnqueuePacket(MarshalAgentMovementTick(50))
	}

	// Handle disconnects after the loops so removal cannot disturb the slice
	// being iterated above.
	for _, p := range disconnect {
		i.removePlayer(p)
		p.OnUserDisconnected()
		p.conn.Close()
	}
}

// processPlayer consumes a player's phase-1 request fields (phase 2 of the
// tick). It must run on the instance actor, and it calls the actor-only impl
// methods directly rather than the blocking deliver wrappers. It returns true
// if the player requested a disconnect, in which case the caller removes and
// closes them.
func (i *Instance) processPlayer(p *Player) bool {
	if p.pendingDisconnect {
		p.pendingDisconnect = false
		return true
	}

	if m := p.moveTo; m != nil {
		p.moveTo = nil
		if err := i.updateRequestedPlayerPos(p, m.x, m.y); err != nil {
			p.log.Error().Err(err).Msg("updateRequestedPlayerPos")
		}
		p.EnqueuePacket(MarshalMoveToPointS2C(p.agentId, m.x, m.y, 0))
	}

	if m := p.movement; m != nil {
		p.movement = nil
		p.facingX = m.facingX
		p.facingY = m.facingY
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
		if err := i.loadSpawnPoint(p); err != nil {
			p.log.Error().Err(err).Msg("loadSpawnPoint")
		}
	}

	if payload := p.loadPlayers; payload != nil {
		p.loadPlayers = nil
		if err := i.loadRequestPlayers(p, *payload); err != nil {
			p.log.Error().Err(err).Msg("loadRequestPlayers")
		}
	}

	if m := p.mapTravel; m != nil {
		p.mapTravel = nil
		i.log.Info().Int("mapId", m.mapId).Msg("MapTravel")
		if err := i.transferPlayerToNewMap(p, m.mapId); err != nil {
			p.log.Error().Err(err).Int("mapId", m.mapId).Msg("MapTravel")
		}
	}

	if c := p.chat; c != nil {
		p.chat = nil
		i.applyChat(p, c)
	}

	return false
}

// applyChat processes a chat message in phase 2 (on the actor). Local chat is
// broadcast to the instance; emotes and commands run here via actor-context
// helpers so they can touch instance state directly.
func (i *Instance) applyChat(p *Player, c *ChatMessage) {
	msg := c.message
	if len(msg) <= 1 {
		return
	}
	p.log.Info().Int("ag", c.agentId).Str("msg", msg).Msg("ChatMessage")

	channel := msg[0]
	remainder := msg[1:]
	switch channel {
	case '!':
		packet := MarshalChatMessageCore(fmt.Sprintf("\u0108\u0107%s\u0001", remainder))
		packet.Merge(MarshalChatMessageLocal(p.playerId, 3))
		if err := i.broadcastGeneric(packet); err != nil {
			p.log.Error().Err(err).Msg("broadcastLocalChat")
		}
	case '/':
		words := strings.Fields(remainder)
		if len(words) == 0 {
			p.SendChatWarning("Invalid command syntax")
			return
		}
		command := words[0]
		args := words[1:]
		if emote, exists := GetEmoteByCommand(command); exists {
			if err := i.broadcastGeneric(MarshalEmote(p.playerId, p.agentId, emote)); err != nil {
				p.log.Error().Err(err).Msg("emote broadcast")
			}
			return
		}
		HandleCommand(i, p, command, remainder, args)
	}
}

// flushPlayers writes each player's buffered outbound packets to their
// connection. A write failure (slow or stuck client) disconnects the player
// rather than stalling the tick.
func (i *Instance) flushPlayers() {
	for _, p := range i.players {
		if err := p.conn.Flush(); err != nil {
			i.log.Warn().Uint64("playerUuid", p.uuid).Err(err).Msg("flush failed, disconnecting player")
			i.removePlayer(p)
			p.conn.Close()
			return
		}
	}
}
