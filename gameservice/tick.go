package gameservice

import (
	"fmt"
	"strings"
)

// gameTick is the actor's fixed-timestep world update. It implements the
// tick-batched two-phase model:
//
//	phase 1 (collect):  drain each player's buffered input into intent fields
//	phase 2 (apply):    consume the intents and mutate world state
//	phase 3 (output):   movement-tick broadcast (folded into the game tick)
//
// Outbound buffers are flushed by the caller (actorLoop) after gameTick
// returns. Intent is consumed within this tick and never queues across ticks.
func (i *Instance) gameTick() {
	// Phase 1: collect intents. Handlers set fields on the player only, so a
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

	// Phase 2: apply intents.
	for _, p := range i.players {
		if i.applyPlayerIntents(p) {
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

// applyPlayerIntents consumes a player's phase-1 intent fields (phase 2 of
// the tick). It must run on the instance actor, and it calls the actor-only
// impl methods directly rather than the blocking deliver wrappers. It returns
// true if the player requested a disconnect, in which case the caller removes
// and closes them.
func (i *Instance) applyPlayerIntents(p *Player) bool {
	if p.pendingDisconnect {
		p.pendingDisconnect = false
		return true
	}

	if m := p.pendingMove; m != nil {
		p.pendingMove = nil
		if err := i.updateRequestedPlayerPos(p, m.x, m.y); err != nil {
			p.log.Error().Err(err).Msg("updateRequestedPlayerPos")
		}
		p.EnqueuePacket(MarshalMoveToPointS2C(p.agentId, m.x, m.y, 0))
	}

	if m := p.pendingMovement; m != nil {
		p.pendingMovement = nil
		p.facingX = m.facingX
		p.facingY = m.facingY
	}

	if r := p.pendingRotate; r != nil {
		p.pendingRotate = nil
	}

	if c := p.pendingMoveCancelled; c != nil {
		p.pendingMoveCancelled = nil
	}

	if p.pendingCancelInteract {
		p.pendingCancelInteract = false
	}

	if it := p.pendingInteract; it != nil {
		p.pendingInteract = nil
		p.SendChatWarning(fmt.Sprintf("missing interaction definition for agent=%d,action=%d", it.agentId, it.action))
		p.log.Debug().Int("target", it.agentId).Int("action", it.action).Msg("InteractAgent")
	}

	if t := p.pendingTarget; t != nil {
		p.pendingTarget = nil
		p.log.Debug().Int("target", t.targetAgentId).Str("playerName", p.name).Msg("UpdateTarget")
	}

	if lid := p.pendingEquip; lid != nil {
		p.pendingEquip = nil
		p.log.Info().Int("itemLocalId", *lid).Msg("EquipItem")
		p.TryEquipItem(*lid)
	}

	if lid := p.pendingDestroy; lid != nil {
		p.pendingDestroy = nil
		p.log.Info().Int("itemLocalId", *lid).Msg("DestroyItem")
		if err := p.itemMgr.RemoveItemByLocalId(*lid); err != nil {
			p.log.Error().Err(err).Int("itemLocalId", *lid).Msg("DestroyItem")
		}
	}

	if p.pendingLoadSpawn {
		p.pendingLoadSpawn = false
		if err := i.loadSpawnPoint(p); err != nil {
			p.log.Error().Err(err).Msg("loadSpawnPoint")
		}
	}

	if payload := p.pendingLoadPlayers; payload != nil {
		p.pendingLoadPlayers = nil
		if err := i.loadRequestPlayers(p, *payload); err != nil {
			p.log.Error().Err(err).Msg("loadRequestPlayers")
		}
	}

	if mapId := p.pendingMapTravel; mapId != nil {
		p.pendingMapTravel = nil
		i.log.Info().Int("mapId", *mapId).Msg("MapTravel")
		if err := i.transferPlayerToNewMap(p, *mapId); err != nil {
			p.log.Error().Err(err).Int("mapId", *mapId).Msg("MapTravel")
		}
	}

	if c := p.pendingChat; c != nil {
		p.pendingChat = nil
		i.applyChat(p, c)
	}

	return false
}

// applyChat processes a chat message in phase 2 (on the actor). Local chat is
// broadcast to the instance; emotes and commands run here via actor-context
// helpers so they can touch instance state directly.
func (i *Instance) applyChat(p *Player, c *chatIntent) {
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
