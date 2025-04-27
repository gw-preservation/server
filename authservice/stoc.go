package AuthService

import GwPacket "gw1/server/gwpacket"

func newRequestResponse(reqNumber int, code int) (resp GwPacket.Out) {

	// Login status code:
	// Missing entries default to "Network error ."

	// 0- OK
	// 1- Reject with no error
	// 3- "Gulid wars was unable to complete the operation"
	// 4- ^
	// 5- Unable to connect to the game server due to a network error
	// 6- Too many connection attempts
	// 7- "Connection to the server was lost"
	// 8- Account territory does not match connection IP territory
	// 9- "Your account is all prepared, but there is no Beta event running right now."
	// 10- Unable to complete the operation
	// 11- Invalid password
	// 12- Too many connection attempts from IP
	// 13- Unable to complete the operation
	// 14- Network login expired - asks for password!
	// 15- Unable to complete the operation
	// 16- ^
	// 17- "Your account is all prepared, but there is no Alpha session running right now."
	// 18- Alpha test servers are presently full.
	// 19- Unable to complete the operation
	// 20- ^
	// 21- Email address not found
	// 24- Unable to complete the operation
	// 25- "Network error ."
	// 26- "Guild Wars was unable to find the server address because a DNS failed"
	// 32- Too many login attempts from your account
	// 33-Network error
	// 34- Your transaction is still in progress.
	// 35- "You have been disconnected from Guild Wars because another client has connected using your account."
	// 40- Unable to complete the operation
	// 41- "You have attempted to create too many characters recently."
	// 42- Unable to complete the operation
	// 43- "You have already created the maximum number of characters for this acccount."
	// 44- "You have made too many server requests in too short a time, and the server has disconnected you."
	// 45- "Your account has been blocked due to unacceptable behaviour. You won't be able to log in again until the block expires."
	// 46- Unable to complete the operation
	// 47- "This version of Guild Wars is no longer supported by the server."
	// 48- Unable to complete the operation
	// 49- "Unable to complete the operation (permission denied)"
	// 50- Unable to complete the operation
	// 56- Unable to complete the operation
	// 57- Unable to complete the operation
	// 58- Guild Wars was unable to connect to any login servers

	// 93 - "Your account is all prepared but there is no event running right now."

	// Guild Battle
	// 94- "Your guild is already involved in a guild battle."
	// 95- "The guild you wish to challenge is already involved in a guild battle."

	// Access Key responses
	// 102 - Invalid access key
	// 103 - Access key in use
	// 104 - Some of the contents of that key are already on the acc, prompt to continue
	// 105 - "Your account already has access to the rights this key provides."
	// 107 - Caracter banned due to offensive name
	// 116 - "Your account is all prepared, but there is no event running right now."
	// 119 - "Your account has already registered this key."
	// 120 - "The access key you have entered was a limited-use key and has already been used."
	// 121 - "The access key you have entered was for an event that has already finished."
	// 122 - "The access key ou have entered has been disabled."

	// Territory change errors
	// 125 - "You have used up all of your allowed territory changes."
	// 126 - "Switching to the chosen territory, or from your current territory, is not supported at this time."
	// 128 - Blocked due to region blocking

	// 133 - The access key you have entered cannot be used to create a new account.

	// 140 - "The Internet (IP) address that you're playing from is already in use."
	// 141 - "Your play time has ended. To continue playing, please purchase Guild Wars."

	// 164 - "Guild Wars has been updated. Please shut down and start again."
	// 165 - "You do not have access to the Guild Wars campaign necessary to select that character."

	// 167 - "Guild Wars is currently down for maintenance."

	// 180 - "You cannot add this key to your account because you already have the features provided by this key."
	// 181 - Guild Wars Official Store is down for maintenance

	// 186 - "The time period to attempt a reconnect has elapsed."

	// 207 - "The matrix card code you have provided is incorrect. Please check your matrix card and try again."
	// 223- "You entered a map in an invalid manner."
	// 224 - IP banned due to repeated rule breaking
	// 225- IP banned due to "believed to be running an open proxy or relay"
	// 226- "You account is blocked for security reasons and requires reauthentication" - references guildwars.co.kr
	// 227 - Legacy message. "We don't recognize your account login information"
	// 244 - Email address not found
	// 247 - Legacy 2FA - asks user to open email link to verify login

	resp = GwPacket.NewOut(0x0003)
	resp.Uint32(reqNumber)
	resp.Uint32(code)
	return
}

func newCharacterSummaryPacket(reqNumber int, charName string, charUUID []byte, mapId int, appearanceBits [8]byte, equipmentData []byte) GwPacket.Out {
	pkt := GwPacket.NewOut(0x007)
	pkt.Uint32(reqNumber)
	// Character UUID bytes
	pkt.Bytes(charUUID)
	pkt.Uint32(0) // Unknown purpose
	pkt.UTF16WithLengthPrefix(charName)
	subBlock := GwPacket.NewOutRaw()
	summaryBlockVersion := 6
	subBlock.Uint16(summaryBlockVersion)
	subBlock.Uint16(mapId)
	// Unknown purpose
	subBlock.Bytes([]byte{0x00, 0x00, 0x00, 0x00})
	// Appearance bits
	subBlock.Bytes(appearanceBits[:])
	subBlock.Uint32(0)
	subBlock.Uint32(0)
	subBlock.Uint32(0) // If this or any of the above 3 are > 0 then we are in Guild Hall
	subBlock.Bytes(equipmentData)
	subBlockBytes := subBlock.GetBytes()
	pkt.Uint16(len(subBlockBytes))
	pkt.Bytes(subBlockBytes)

	return pkt
}

func newAccountExtraInfo_0014(requestNumber int, accountUUID []byte, activeCharUUID []byte, territoryCode int, readEula bool) GwPacket.Out {
	pkt := GwPacket.NewOut(0x0014)
	pkt.Uint32(requestNumber)
	pkt.Bytes([]byte{0x00, 0x00, 0x00, 0x00})
	// 0x11, 0x00 looks to be like EULA/enrolment info?
	// If it's absent then you have to agree to EULA, and even when agreeing, you get an unauthorized message!
	pkt.Bytes([]byte{0x11, 0x00})
	pkt.Uint32(requestNumber)
	// 'territory' identifier. only 0-6 are supported, higher gives assertion crash.
	// Territories:
	// 0- America
	// 1- Korea
	// 2- Europe
	// 3- Taiwan
	// 4- Japan
	// 5- China
	// 6- China
	pkt.Uint32(territoryCode)
	pkt.Uint32(4) // What's this? Language maybe?
	// 00 = Only EoTN
	// 01 = Proph + EoTN
	// 02 = Factions + EoTN
	// 03 = Proph + Factions + EoTN
	// 04 = NF + EoTN
	// 05 = Proph + NF + EoTN
	// 06 = Factions + NF + EoTN
	// 07 = Proph + Factions + NF + EoTN

	// 08 = Only EoTN
	// 09 = Proph + EoTN
	// 10 = Factions + EoTN
	// 11 = Proph + Factions + EoTN
	// 12 = NF + EoTN

	pkt.Bytes([]byte{
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	pkt.Bytes(accountUUID)
	pkt.Bytes(activeCharUUID) // Active character
	/*
		Dword(3)
		UnhandledField (type=14)
		Byte(4)
		Dword(100663552)

		VarBytes(4) 01 00 06 00
		Byte(24)
		Dword(1)
	*/
	pkt.Uint32(3)
	pkt.Uint16(4) // 4 bytes extra for account unlocks
	pkt.Uint16(1) // Type 1 = Extra Char Slots
	pkt.Uint16(6)

	if readEula {
		pkt.Uint8(0x18) // EULA revision read?
	} else {
		pkt.Uint8(0)
	}
	pkt.Uint32(1) // Unknown
	return pkt
}

func newAccountBinaryInfo_0016(requestNumber int) GwPacket.Out {
	pkt := GwPacket.NewOut(0x0016)
	pkt.Uint32(requestNumber)
	accountInfoBlock := []byte{}
	pkt.Uint16(len(accountInfoBlock))
	pkt.Bytes(accountInfoBlock)
	return pkt
}

func newServerSeed(xoredRandomBytes []byte) *GwPacket.Out {
	p := GwPacket.NewOutRaw()
	p.Uint8(1)
	p.Uint8(len(xoredRandomBytes) + 2)
	p.Bytes(xoredRandomBytes)
	return &p
}

func newSessionInfo(salt int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(1)
	resp.Uint32(salt)
	resp.Uint32(0xffffffff)
	return
}

func newInstanceServerInfo(requestNumber int, worldId int, mapId int, playerId int) (resp GwPacket.Out) {
	resp = GwPacket.NewOut(0x0009)
	resp.Uint32(requestNumber)
	resp.Uint32(worldId)
	resp.Uint32(mapId)
	resp.Bytes([]byte{0x02, 0x00})             // AF_INET
	resp.Bytes([]byte{0x17, 0xe0})             // port (6112)
	resp.Bytes([]byte{0xc0, 0xa8, 0x01, 0x50}) // 192.168.1.80
	resp.Bytes([]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	resp.Uint32(playerId)
	return
}
