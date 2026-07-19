package GameService

import GwPacket "gw1/server/gwpacket"

type EmoteDefinition struct {
	emoteId      int
	messageUnits []uint16
}

var emoteMap map[string]EmoteDefinition

func init() {
	emoteMap = map[string]EmoteDefinition{
		"dance": {emoteId: 831757499, messageUnits: []uint16{0x79d, 0x10d, 0x101}},
		"cheer": {emoteId: 909459641, messageUnits: []uint16{0x79a, 0x10d, 0x101}},
	}
}

func GetEmoteByCommand(entry string) (EmoteDefinition, bool) {
	def, exists := emoteMap[entry]
	return def, exists
}

func MarshalEmote(forAgentId int, emote EmoteDefinition) GwPacket.Out {
	// First, we send a chat message so the emote appears in the chat log.
	resp := GwPacket.NewOut(0x5c)
	resp.Uint16(len(emote.messageUnits))
	for _, unit := range emote.messageUnits {
		resp.Uint16(int(unit))
	}
	resp.Merge(MarshalChatMessageServer(6))

	// Next, we send the emote itself, which is a series of attribute updates.
	resp.Merge(MarshalAgentAttrUpdateInt(23, forAgentId, 8))
	resp.Merge(MarshalAgentAttrUpdateInt(28, forAgentId, emote.emoteId))
	resp.Merge(MarshalAgentAttrUpdateInt(8, forAgentId, 0))
	return resp
}
