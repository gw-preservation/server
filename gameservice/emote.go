package GameService

import GwPacket "gw1/server/gwpacket"

type EmoteDefinition struct {
	datEmoteId  int
	datStringId int
}

var emoteMap map[string]EmoteDefinition

func init() {
	emoteMap = map[string]EmoteDefinition{
		// Missing:
		// /fame, /rank: Hero rank
		// /zaishen /zrank: Zaishen rank
		// /guild, /ladder, /champion: guild ladder ranking
		// /roll <max>: Dice roll, disabled in towns, outpost or PvP areas
		"agree":       {datEmoteId: 832368954, datStringId: 0x791},
		"attention":   {datEmoteId: 3548536382, datStringId: 0x792},
		"beckon":      {datEmoteId: 2667954541, datStringId: 0x793},
		"beg":         {datEmoteId: 805381513, datStringId: 0x794},
		"scare":       {datEmoteId: 805471443, datStringId: 0x795},
		"boo":         {datEmoteId: 805471443, datStringId: 0x795}, // dup
		"bored":       {datEmoteId: 825936337, datStringId: 0x796},
		"bow":         {datEmoteId: 805556612, datStringId: 0x797},
		"bowhead":     {datEmoteId: 1277368538, datStringId: 0x798},
		"head":        {datEmoteId: 1277368538, datStringId: 0x798}, // dup
		"breath":      {datEmoteId: 1957468924, datStringId: 0x799},
		"catchbreath": {datEmoteId: 1957468924, datStringId: 0x799}, // dup
		"cheer":       {datEmoteId: 909459641, datStringId: 0x79a},
		"congrates":   {datEmoteId: 910119833, datStringId: 0x79b},
		"cough":       {datEmoteId: 852277915, datStringId: 0x79c},
		"dance":       {datEmoteId: 831757499, datStringId: 0x79d},
		"doh":         {datEmoteId: 805398487, datStringId: 0x79e},
		"doubletake":  {datEmoteId: 2142788779, datStringId: 0x79f},

		"drum":      {datEmoteId: 918042160, datStringId: 0x7a0},
		"drums":     {datEmoteId: 918042160, datStringId: 0x7a0}, // dup
		"clap":      {datEmoteId: 809229482, datStringId: 0x7a1},
		"excited":   {datEmoteId: 912210970, datStringId: 0x7a1},
		"fistshake": {datEmoteId: 1956223750, datStringId: 0x7a3},
		"flex":      {datEmoteId: 811237106, datStringId: 0x7a4},
		"flute":     {datEmoteId: 836325460, datStringId: 0x7a5},
		"goteam":    {datEmoteId: 2435046536, datStringId: 0x7a6},
		"encourage": {datEmoteId: 2435046536, datStringId: 0x7a6}, // dup
		"guitar":    {datEmoteId: 3179025259, datStringId: 0x7a7},
		"airguitar": {datEmoteId: 3179025259, datStringId: 0x7a7}, // dup
		"helpme":    {datEmoteId: 809348093, datStringId: 0x7a8},
		"highfive":  {datEmoteId: 806612588, datStringId: 0x7a9},
		"five":      {datEmoteId: 806612588, datStringId: 0x7a9}, // dup
		"jump":      {datEmoteId: 809368241, datStringId: 0x7aa},
		"kneel":     {datEmoteId: 870844389, datStringId: 0x7ab},
		"laugh":     {datEmoteId: 852271222, datStringId: 0x7ac},
		"moan":      {datEmoteId: 808671594, datStringId: 0x7ad},
		"no":        {datEmoteId: 805313525, datStringId: 0x7ae},
		"pickme":    {datEmoteId: 1470797158, datStringId: 0x7af},
		"point":     {datEmoteId: 924623173, datStringId: 0x7b0},
		"ponder":    {datEmoteId: 3200618694, datStringId: 0x7b1},
		"pout":      {datEmoteId: 810581882, datStringId: 0x7b2},
		"rock":      {datEmoteId: 807856520, datStringId: 0x7b3},
		"paper":     {datEmoteId: 909577884, datStringId: 0x7b3},
		"scissors":  {datEmoteId: 3429754055, datStringId: 0x7b3},
		"scis":      {datEmoteId: 3429754055, datStringId: 0x7b3}, // dup
		"ready":     {datEmoteId: 947747925, datStringId: 0x7b4},
		"roar":      {datEmoteId: 809791073, datStringId: 0x7b5},
		"salute":    {datEmoteId: 1518743142, datStringId: 0x7b6},
		"scratch":   {datEmoteId: 1859663540, datStringId: 0x7b7},
		"shoo":      {datEmoteId: 809106570, datStringId: 0x7b8},
		"sigh":      {datEmoteId: 807342884, datStringId: 0x7b9},
		"sorry":     {datEmoteId: 951585314, datStringId: 0x7ba},
		"taunt":     {datEmoteId: 924750225, datStringId: 0x7bb},
		"rude":      {datEmoteId: 924750225, datStringId: 0x7bb}, // dup
		"voilin":    {datEmoteId: 2636188942, datStringId: 0x7bc},
		"wave":      {datEmoteId: 806608701, datStringId: 0x7bd},
		"yawn":      {datEmoteId: 808908310, datStringId: 0x7be},
		"yes":       {datEmoteId: 805515833, datStringId: 0x7bf},
	}
}

func GetEmoteByCommand(entry string) (EmoteDefinition, bool) {
	def, exists := emoteMap[entry]
	return def, exists
}

func MarshalEmote(forAgentId int, emote EmoteDefinition) GwPacket.Out {
	// First, we send a chat message so the emote appears in the chat log.
	resp := GwPacket.NewOut(0x5c)
	resp.Uint16(3)
	resp.Uint16(emote.datStringId)
	resp.Uint16(0x10d)
	resp.Uint16(0x100 | forAgentId)
	resp.Merge(MarshalChatMessageServer(6))

	// Next, we send the emote itself, which is a series of attribute updates.
	resp.Merge(MarshalAgentAttrUpdateInt(23, forAgentId, 8))
	resp.Merge(MarshalAgentAttrUpdateInt(28, forAgentId, emote.datEmoteId))
	resp.Merge(MarshalAgentAttrUpdateInt(8, forAgentId, 0))
	return resp
}
