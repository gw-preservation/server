package gameservice

import (
	"fmt"
	"strings"
)

type commandHandler func(p *Player, args []string) bool

func HandleCommand(p *Player, command string, fullInput string, args []string) bool {
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
		return handler(p, args)
	}
	p.SendChatWarning(fmt.Sprintf("Unknown command: %s", fullInput))
	return false
}

func handleSpeedCommand(p *Player, args []string) bool {
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
	p.baseSpeed = speed
	p.EnqueuePacket(MarshalAgentUpdateSpeedBase(p.agentId, p.baseSpeed))
	return true
}

func handleEquipCommand(p *Player, args []string) bool {
	if len(args) < 1 {
		p.SendChatWarning("Usage: /e <profession>")
		return false
	}
	p.equipTest(args[0])
	return true
}

func handleMotdCommand(p *Player, args []string) bool {
	p.EnqueuePacket(MarshalMessageOfTheDay("\u0108\u0107Test <c=@ItemRare>message\u0001"))
	return true
}

func handleGvCommand(p *Player, args []string) bool {
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
	p.EnqueuePacket(MarshalAgentAttrUpdateInt(msgType, p.agentId, value))
	return true
}

func handleColorCommand(p *Player, args []string) bool {
	p.SendChatColorTest()
	return true
}

func handleTravelCommand(p *Player, args []string) bool {
	if p.connectedInstance == nil {
		p.SendChatWarning("You are not in a game instance")
		return false
	}
	if len(args) < 1 {
		p.SendChatWarning("Usage: /travel <mapId> or /travel \"<map_name>\"")
		return false
	}

	var newMapId int
	nParsed, err := fmt.Sscanf(args[0], "%d", &newMapId)
	if nParsed == 0 || err != nil {
		var ok bool
		newMapId, ok = GetMapIdForNameCaseInsensitive(strings.ReplaceAll(strings.Join(args, " "), "\"", ""))
		if !ok || newMapId == 0 {
			p.log.Error().Err(err).Msg("failed to find map by id or name")
			p.SendChatWarning("Unable to find map.")
			return false
		}
	}
	p.log.Info().Int("newMapId", newMapId).Msg("travel command")
	if !HasInstanceDefinitionForMapId(newMapId) {
		p.SendChatWarning(fmt.Sprintf("Map ID %d has no definition data", newMapId))
		return false
	}
	err = p.connectedInstance.TransferPlayerToNewMap(p, newMapId)
	if err != nil {
		p.log.Error().Err(err).Int("newMapId", newMapId).Msg("failed to transfer player to new map")
		return false
	}
	return true
}
