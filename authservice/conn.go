package AuthService

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/crypt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	PortalService "gw1/server/portalservice"
	"net"

	"github.com/rs/zerolog"
)

type State int

const (
	StateReadClientVersion State = iota
	StateReadClientSeed
	StateCharacterScreen
	StateCreateCharacter
	StateInInstance
)

type ASConn struct {
	socket *net.TCPConn
	state  State
	enc    *rc4.Cipher
	dec    *rc4.Cipher
	out    GwPacket.Out
	log    zerolog.Logger
	acc    db.Account
}

func NewASConn(socket *net.TCPConn, logCtx zerolog.Logger) *ASConn {
	conn := ASConn{
		socket: socket,
		state:  StateReadClientVersion,
		log:    logCtx.With().Str("srv", "auth").Logger(),
		out:    GwPacket.NewOutRaw(),
	}
	conn.log.Info().Msg("new client")
	return &conn
}

func (conn *ASConn) DecryptBytes(data []byte) {
	if conn.dec != nil {
		conn.dec.XORKeyStream(data, data)
	}
}

func (conn *ASConn) HandleBytes(data []byte) (int, error) {
	inPkt := GwPacket.NewIn(data)
	if conn.state == StateReadClientVersion {
		return conn.onClientVersion(&inPkt)
	} else if conn.state == StateReadClientSeed {
		return conn.onClientSeed(&inPkt)
	} else {
		return conn.onPacket(&inPkt)
	}
}

func (conn *ASConn) onComputerInfo(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalComputerInfo(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalComputerInfo: %w", err)
	}

	conn.log.Info().Str("user", payload.userName).Str("name", payload.computerName).Msg("ComputerUserInfo")
	return in.Position(), nil
}

func (conn *ASConn) onClientHashInfo(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalClientHashInfo(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalClientHashInfo: %w", err)
	}
	if payload.clientVersion != 37600 { //37587 {
		// Wrong client version
		return 0, fmt.Errorf("bad client version %d", payload.clientVersion)
	}
	conn.log.Debug().
		Str("op", fmt.Sprintf("%04x", in.Opcode())).
		Int("clientVersion", payload.clientVersion).
		//Hex("unkHash", payload.unkHash[:]). // Maybe memory hash / hash of DH keys?
		Msg("")

	conn.EnqueuePacket(MarshalSessionSaltInfo(0x51c6ea1d, 0xffffffff)) // Salt unknown
	return in.Position(), nil
}

func (conn *ASConn) on8000(in *GwPacket.In) (int, error) {
	_, err := UnmarshalUnknown8000(in)
	if err != nil {
		return 0, fmt.Errorf("Unmarshal8000: %w", err)
	}
	return in.Position(), nil
}

func (conn *ASConn) on8023(in *GwPacket.In) (int, error) {
	_, err := UnmarshalUnknown8023(in)
	if err != nil {
		return 0, fmt.Errorf("Unmarshal8023: %w", err)
	}
	conn.EnqueuePacket(MarshalUnknown0000(0x8002e647, 0x17))
	return in.Position(), nil
}

func (conn *ASConn) on8038_GetAccountInfo(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalGetAccountInfo(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalGetAccountInfo: %w", err)
	}

	conn.log.Info().Hex("uuid1", payload.uuid1[:]).Hex("gameToken", payload.gameTokenFromPortalService[:]).Str("unkString", payload.unk1).Msg("GetAccountInfo")
	// Validate connection token
	tokenStr := db.UUIDStr(payload.gameTokenFromPortalService[:])
	accountId, ok := PortalService.ValidateConnectionToken(tokenStr)
	if !ok {
		// Bad connection token!
		return 0, fmt.Errorf("invalid GameConnectionToken")
	}
	// Re-retrieve account from DB:
	conn.acc, ok = db.GetFullAccountByID(accountId)
	if !ok {
		// Account does not exist though a token was generated to connect to it?
		return 0, fmt.Errorf("no such account during GetAccountInfo token verification")
	}

	// Now send all characters belonging to the account:
	lastCharUUID := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	for _, char := range conn.acc.Characters {
		subBlock := GwPacket.NewOutRaw()
		summaryBlockVersion := 6
		subBlock.Uint16(summaryBlockVersion)
		subBlock.Uint16(int(char.LastOutpostID))
		// Unknown purpose
		subBlock.Uint32(0)
		// Appearance bits
		conn.log.Info().Uint32("appearanceBytes", char.AppearanceBits).Msg("Appearance")
		subBlock.Uint32(int(char.AppearanceBits))
		subBlock.Bytes([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // Guild Hall ID
		var bits16 uint16
		bits16 |= uint16(1 & 0xF)                            // CampaignType
		bits16 |= uint16(char.Level&0x1F) << 4               // Level
		bits16 |= uint16(char.ProfessionSecondary&0xF) << 10 // SecondaryProfession
		bits16 |= uint16(1&0x3) << 14                        // HelmStatus
		subBlock.Uint16(int(bits16))

		subBlock.Uint16(0) // H001E
		subBlock.Uint8(0)  // number_of_pieces
		subBlock.Bytes([]byte{0xDD, 0xDD, 0xDD, 0xDD})

		subBlockBytes := subBlock.GetBytes()

		conn.EnqueuePacket(MarshalCharacterSummary(
			payload.reqNumber,
			char.UUID,
			0,
			char.Name,
			subBlockBytes,
		))
		lastCharUUID = char.UUID
	}
	/* Note, from a brand new Masterpiece acc, seeing Eula for first time and pressing accept:
		<<-- [+3.239s] 0x16 {
	    Dword(1)
	    VarBytes(0)
	}
	<<-- [+3.239s] 0x14 {
	    Dword(1)
	    Dword(0)
	}
	<<-- [+3.239s] 0x11 {
	    Dword(1)
	    Dword(0)
	    Dword(4)
	    Blob(8) => 3f 00 00 00 00 00 00 00
	    Blob(8) => 80 3f 02 00 03 00 0c 00
	    Blob(16) => 05 2f 60 5b 4f fd 82 47 ba af 6e 03 45 b6 0c 1d
	    Blob(16) => 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
	    Dword(3)
	    VarBytes(8) 01 00 06 00 57 00 01 00
	    Byte(0)  // This is set to 0x18 (24) once EULA is agreed
	    Dword(0)
	}
	<<-- [+3.239s] 0x3 {
	    Dword(1)
	    Dword(0)
	}
	-->> [+6.927s] 0x26 {  // This is only sent when EULA is clicked Agree
	    Byte(24)
	}
	-->> [+6.927s] 0x35 {
	    Dword(2)
	}
	<<-- [+7.011s] 0x3 {
	    Dword(2)
	    Dword(181)
	}
	-->> [+22.809s] 0xe {
	    Dword(0)
	}
	*/
	conn.EnqueuePacket(MarshalAccountBinaryInfo(payload.reqNumber, []byte{}))
	conn.EnqueuePacket(MarshalAccountExtraInfoStart(payload.reqNumber, 0))
	conn.EnqueuePacket(MarshalAccountExtraInfo(
		payload.reqNumber,
		// Territory
		// 0- America
		// 1- Korea
		// 2- Europe
		// 3- Taiwan
		// 4- Japan
		// 5- China
		// 6- China
		2,
		// Language
		4,
		// Campaigns purchased - 1 = Proph only.
		[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		conn.acc.UUID, lastCharUUID, 3, []byte{0x01, 0x00, 0x06, 0x00}, 0x18, 1,
	))
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))

	return in.Position(), nil
}

func (conn *ASConn) on8035_AskServerResponse(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalAskServerResponse(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalAskServerResponse: %w", err)
	}
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0xb5))

	return in.Position(), nil
}

func (conn *ASConn) on8016(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalLanguageInfo(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalLanguageInfo: %w", err)
	}
	conn.log.Info().
		Int("languageCode", payload.languageCode).
		Msg("LanguageUpdate")
	return in.Position(), nil
}

func (conn *ASConn) on800d_Disconnect(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalDisconnect(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalDisconnect: %w", err)
	}
	conn.log.Info().
		Int("errCode", payload.errorCode).Msg("Disconnect")
	conn.socket.Close()
	return in.Position(), nil
}

func (conn *ASConn) on8029_LoginCharacter(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalLoginCharacter(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalLoginCharacter: %w", err)
	}
	//if conn.state != StateCharacterScreen {
	//	return 0, fmt.Errorf("LoginCharacter: bad client state %v", conn.state)
	//}
	// For creating a new character, we get this:
	// 2:11PM INF LoginCharacter mapId=0 srv=auth unk2=11 unk4=0 unk5=4 unk6=0
	// For logging an existing character, we get this:
	// 2:12PM INF LoginCharacter mapId=165 srv=auth unk2=3 unk4=0 unk5=4 unk6=0
	conn.log.Info().
		Int("unk2", payload.unk1).
		Int("mapId", payload.mapId).
		Int("unk4", payload.unk2).
		Int("unk5", payload.unk3).
		Int("unk6", payload.unk4).
		Msg("LoginCharacter")

	if payload.mapId == 0 {
		conn.log.Info().Msg("State = StateCreateCharacter")
		conn.state = StateCreateCharacter
	} else {
		conn.state = StateInInstance
	}
	worldId := 0x10101010
	playerId := 0xbebafeca
	conn.EnqueuePacket(MarshalInstanceServerInfo(payload.reqNumber, worldId, payload.mapId, []byte{
		0x02, 0x00, // AF_INET
		0x17, 0xe0, // Port 6112
		0xc0, 0xa8, 0x01, 0x50, // 192.168.1.80
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, playerId,
	))

	return in.Position(), nil
}

func (conn *ASConn) on801c_AddAccessKey(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalAddAccessKey(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalAddAccessKey: %w", err)
	}
	conn.log.Info().Str("key", payload.key).Msg("AddAccessKey")
	// 0 = OK
	// 102 = InvalidAccessKey
	// 103 = AccessKeyInUse
	// 105 = AccessKeyNotNeeded
	// 119 = AccessKeyAlreadyAppliedByYourAccount
	// 122 = AccessKeyDisabled
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return in.Position(), nil
}

func (conn *ASConn) on800a_SetActiveCharacter(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalSetActiveCharacter(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalSetActiveCharacter: %w", err)
	}
	conn.log.Info().Str("charName", payload.charName).Msg("SetActiveCharacter")
	// Client sends this with empty charName if the active char was already selected upon a login char request

	/*if payload.charName == "Char B" {
		conn.EnqueuePacket(newAccountExtraInfo_0014(payload.reqNumber, char2UUID[:], 4, true))
	} else {
		conn.EnqueuePacket(newAccountExtraInfo_0014(payload.reqNumber, char1UUID[:], 4, true))
	}*/
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return in.Position(), nil
}

func (conn *ASConn) on800e_SetPlayerOnlineVisibilityStatus(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalSetPlayerOnlineVisibilityStatus(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalSetPlayerOnlineVisibilityStatus: %w", err)
	}
	conn.log.Info().Int("newVisibility", payload.visibility).Msg("SetPlayerOnlineVisibilityStatus")
	return in.Position(), nil
}

func (conn *ASConn) on8021_UpdateSettingsLength(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalUpdateSettingsLength(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalUpdateSettingsLength: %w", err)
	}
	conn.log.Info().Int("unk1", payload.unk1).Int("unk2", payload.unk2).Msg("UpdateSettingsLength")
	return in.Position(), nil
}

func (conn *ASConn) on8020_UpdateSettings(in *GwPacket.In) (int, error) {
	_, err := UnmarshalUpdateSettings(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalUpdateSettings: %w", err)
	}
	//conn.log.Info().Int("unk1", payload.unk1).Hex("settings", payload.settings).Msg("UpdateSettings")
	return in.Position(), nil
}

func (conn *ASConn) onPacket(in *GwPacket.In) (consumed int, err error) {
	op, _ := in.Uint16()
	switch op {
	case 0x8001:
		consumed, err = conn.onComputerInfo(in)
	case 0x8002:
		consumed, err = conn.onClientHashInfo(in)
	case 0x8000:
		consumed, err = conn.on8000(in)
	case 0x800a:
		consumed, err = conn.on800a_SetActiveCharacter(in)
	case 0x8016:
		consumed, err = conn.on8016(in)
	case 0x801c:
		consumed, err = conn.on801c_AddAccessKey(in)
	case 0x8023:
		consumed, err = conn.on8023(in)
	case 0x8029:
		consumed, err = conn.on8029_LoginCharacter(in)
	case 0x8038:
		consumed, err = conn.on8038_GetAccountInfo(in)
	case 0x8035:
		consumed, err = conn.on8035_AskServerResponse(in)
	case 0x800e:
		consumed, err = conn.on800e_SetPlayerOnlineVisibilityStatus(in)
	case 0x800d: //, 0x800e:
		consumed, err = conn.on800d_Disconnect(in)
	case 0x8021:
		consumed, err = conn.on8021_UpdateSettingsLength(in)
	case 0x8020:
		consumed, err = conn.on8020_UpdateSettings(in)
	default:
		// Unexpected opcode!
		return 0, fmt.Errorf("[%04x] UNEXPECTED; len=%d", op, in.Remaining())
	}
	//conn.log.Debug().Str("op", fmt.Sprintf("%04x", op)).Int("consumed", consumed).Int("remaining", in.Remaining()).Msg("")
	if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}
	return
}

func (conn *ASConn) onClientVersion(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalClientVersionInfo(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalClientVersionPacket: %w", err)
	}
	conn.log.Info().
		Int("version", payload.clientVersion).
		Msg("ClientVersion")
	conn.state = StateReadClientSeed
	return pkt.Position(), nil
}

func (conn *ASConn) onClientSeed(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalClientSeed(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalClientSeed: %w", err)
	}
	rc4Key, publicBytes := crypt.GenerateEncryptionKey([64]byte(payload.seedBytes))

	// Reply with ServerSeed, contents being the xored bytes.
	seedOut := MarshalServerSeed(publicBytes[:])
	conn.WritePacket(&seedOut)
	conn.state = StateCharacterScreen

	// Immediately after, enable RC4!
	conn.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 66, fmt.Errorf("error creating rc4 encrypter: %s", err)
	}
	conn.dec, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 66, fmt.Errorf("error creating rc4 decrypter: %s", err)
	}

	conn.log.Info().Msg("ClientSeed")

	return 66, nil
}

func (conn *ASConn) Read(buf []byte) (int, error) {
	return conn.socket.Read(buf)
}

func (conn *ASConn) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	if conn.enc != nil {
		conn.enc.XORKeyStream(bts, bts)
	}
	_, err := conn.socket.Write(bts)
	return err
}

func (conn *ASConn) Close() {
	conn.socket.Close()
}

func (conn *ASConn) EnqueuePacket(packet GwPacket.Out) {
	conn.out.Merge(packet)
}
