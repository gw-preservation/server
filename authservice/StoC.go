//lint:file-ignore U1000 Fields are not unused
//go:generate go run ../cmd/codegen/main.go s2c fmt
//go:generate go fmt

package AuthService

type VarByte []byte

// opcode: 0x0003
/*
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
*/
type RequestResponse struct {
	reqNumber    int // wire:uint32
	responseCode int // wire:uint32
}

// opcode: 0x0007
type CharacterSummary struct {
	reqNumber int    // wire:uint32
	charUUID  []byte // len:16
	unk1      int    // wire:uint32
	charName  string
	summary   VarByte
}

// opcode: 0x0014
type AccountExtraInfoStart struct {
	reqNumber int // wire:uint32
	unk1      int // wire:uint32
}

// opcode: 0x0011
type AccountExtraInfo struct {
	reqNumber      int    // wire:uint32
	territoryCode  int    // wire:uint32
	languageCode   int    // wire:uint32
	unk1           []byte // len:8
	unk2           []byte // len:8
	accountUUID    []byte // len:16
	activeCharUUID []byte // len:16
	unk3           int    // wire:uint32
	entitlements   VarByte
	eulaByte       int // wire:uint8
}

// opcode: 0x0016
type AccountBinaryInfo struct {
	reqNumber  int // wire:uint32
	binaryData VarByte
}

// opcode: 0x1601
type ServerSeed struct {
	xoredRandomBytes []byte // len:20
}

// opcode: 0x0001
type SessionSaltInfo struct {
	salt int // wire:uint32
	unk1 int // wire:uint32
}

// opcode: 0x0009
type InstanceServerInfo struct {
	reqNumber  int    // wire:uint32
	worldHash  int    // wire:uint32
	mapId      int    // wire:uint32
	socketData []byte // len:24
	playerHash int    // wire:uint32
}

// opcode: 0x0000
type Unknown0000 struct {
	unk1 int // wire:uint32
	unk2 int // wire:uint32
}
