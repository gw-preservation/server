package GameService

import "fmt"

type commandHandler func(p *Player, args []string) bool

var commandHandlers = map[string]commandHandler{
	"speed":  handleSpeedCommand,
	"e":      handleEquipCommand,
	"motd":   handleMotdCommand,
	"gv":     handleGvCommand,
	"color":  handleColorCommand,
	"travel": handleTravelCommand,
}

func HandleCommand(p *Player, command string, fullInput string, args []string) bool {
	if handler, exists := commandHandlers[command]; exists {
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
	p.EnqueuePacket(MarshalAgentUpdateSpeedBase(p.agentId, speed))
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

func handleColorCommand(p *Player, args []string) bool {
	p.SendChatColorTest()
	return true
}

func handleTravelCommand(p *Player, args []string) bool {
	if len(args) < 1 {
		p.SendChatWarning("Usage: /travel <mapId> or /travel \"<map_debug_name>\"")
		return false
	}

	var newMapId int
	nParsed, err := fmt.Sscanf(args[0], "%d", &newMapId)
	if nParsed == 0 || err != nil {
		// maybe it's a name instead of an ID
		var ok bool
		newMapId, ok = GetMapIdForName(args[0])
		if !ok || newMapId == 0 {
			p.log.Error().Err(err).Msg("failed to find map by id or debug name")
			return false
		}
	}
	p.log.Info().Int("newMapId", newMapId).Msg("travel command")
	// Is it a valid map?
	if !HasInstanceDefinitionForMapId(newMapId) {
		p.SendChatWarning(fmt.Sprintf("Map ID %d is not valid", newMapId))
		return false
	}
	// Transfer player to new map
	err = p.connectedInstance.TransferPlayerToNewMap(p, newMapId)
	if err != nil {
		p.log.Error().Err(err).Int("newMapId", newMapId).Msg("failed to transfer player to new map")
		return false
	}
	return true
}
