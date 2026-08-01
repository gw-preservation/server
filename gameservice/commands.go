package gameservice

import (
	"fmt"
	"strings"
)

// commandHandler runs on the instance actor (chat/commands are applied in
// phase 2 of the game tick). i is the actor-owned instance: world-mutating
// commands (e.g. travel) call its impls directly — never the blocking
// deliver wrappers, which would deadlock on the actor. Handlers that only
// touch the player or send packets ignore i.
type commandHandler func(i *Instance, p *Player, args []string) bool

func HandleCommand(i *Instance, p *Player, command string, fullInput string, args []string) bool {
	var handler commandHandler
	switch command {
	case "speed":
		handler = handleSpeedCommand
	case "e":
		handler = handleEquipCommand
	case "motd":
		handler = handleMotdCommand
	case "gv":
		handler = handleGvCommand
	case "color":
		handler = handleColorCommand
	case "travel":
		handler = handleTravelCommand
	}
	if handler != nil {
		return handler(i, p, args)
	}
	p.SendChatWarning(fmt.Sprintf("Unknown command: %s", fullInput))
	return false
}

func handleSpeedCommand(i *Instance, p *Player, args []string) bool {
	if len(args) < 1 {
		p.SendChatWarning("Usage: /speed <speed>")
		return false
	}
	var speed float32
	nParsed, err := fmt.Sscanf(args[0], "%f", &speed)
	if nParsed == 0 || err != nil {
		p.SendChatWarning("Usage: /speed <speed>")
		return false
	}
	p.EnqueuePacket(MarshalAgentUpdateSpeedBase(p.agentId, speed))
	return true
}

func handleEquipCommand(i *Instance, p *Player, args []string) bool {
	if len(args) < 1 {
		p.SendChatWarning("Usage: /e <profession>")
		return false
	}
	p.equipTest(args[0])
	return true
}

func handleMotdCommand(i *Instance, p *Player, args []string) bool {
	p.EnqueuePacket(MarshalMessageOfTheDay("\u0108\u0107Test <c=@ItemRare>message\u0001"))
	return true
}

func handleGvCommand(i *Instance, p *Player, args []string) bool {
	if len(args) < 2 {
		p.SendChatWarning("Usage: /gv <typ> <value>")
		return false
	}
	var msgType int
	nParsed, err := fmt.Sscanf(args[0], "%d", &msgType)
	if nParsed == 0 || err != nil {
		p.SendChatWarning("Usage: /gv <typ> <value>")
		return false
	}
	var value int
	nParsed, err = fmt.Sscanf(args[1], "%d", &value)
	if nParsed == 0 || err != nil {
		p.SendChatWarning("Usage: /gv <typ> <value>")
		return false
	}
	p.log.Info().Int("msgType", msgType).Int("value", value).Msg("Sending GenericValue message")
	// 6 (AddEffect):
	//   24 = Black effect from eyes
	//   20 = Blue swirly
	//   19 = Orb thingy
	//   18 = Unknown thingy
	//   17 = Big blue ring
	//   15 = Blue swirly
	p.EnqueuePacket(MarshalAgentAttrUpdateInt(msgType, p.agentId, value))
	return true
}

func handleColorCommand(i *Instance, p *Player, args []string) bool {
	p.SendChatColorTest()
	return true
}

// handleTravelCommand runs on the instance actor (chat is applied in phase 2),
// so it calls the transferPlayerToNewMap impl directly rather than the
// blocking deliver wrapper, which would deadlock on its own actor.
func handleTravelCommand(i *Instance, p *Player, args []string) bool {
	if len(args) < 1 {
		p.SendChatWarning("Usage: /travel <mapId> or /travel \"<map_name>\"")
		return false
	}

	var newMapId int
	nParsed, err := fmt.Sscanf(args[0], "%d", &newMapId)
	if nParsed == 0 || err != nil {
		// maybe it's a name instead of an ID
		var ok bool
		newMapId, ok = GetMapIdForNameCaseInsensitive(strings.ReplaceAll(strings.Join(args, " "), "\"", ""))
		if !ok || newMapId == 0 {
			p.log.Error().Err(err).Msg("failed to find map by id or name")
			p.SendChatWarning("Unable to find map.")
			return false
		}
	}
	p.log.Info().Int("newMapId", newMapId).Msg("travel command")
	// Is it a valid map?
	if !HasInstanceDefinitionForMapId(newMapId) {
		p.SendChatWarning(fmt.Sprintf("Map ID %d has no definition data", newMapId))
		return false
	}
	// Transfer player to new map
	err = i.transferPlayerToNewMap(p, newMapId)
	if err != nil {
		p.log.Error().Err(err).Int("newMapId", newMapId).Msg("failed to transfer player to new map")
		return false
	}
	return true
}
