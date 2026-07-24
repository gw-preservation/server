package AuthService

import (
	"bytes"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"gw1/server/crypt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	PortalService "gw1/server/portalservice"
)

type packetHandler func(*ASConn, *GwPacket.In) (int, error)

func wrap[T any](
	unmarshal func(*GwPacket.In) (T, error),
	handler func(*ASConn, *T) error,
) packetHandler {
	return func(conn *ASConn, in *GwPacket.In) (int, error) {
		payload, err := unmarshal(in)
		if err != nil {
			return 0, err
		}

		err = handler(conn, &payload)
		if err != nil {
			return 0, err
		}
		return in.Position(), nil
	}
}

var packetHandlers = map[int]packetHandler{
	0x4200: wrap(UnmarshalClientSeed, (*ASConn).onClientSeed),
	0x0400: wrap(UnmarshalClientVersionInfo, (*ASConn).onClientVersion),
	0x0000: wrap(UnmarshalUnknown8000, (*ASConn).on8000),
	0x0001: wrap(UnmarshalComputerInfo, (*ASConn).onComputerInfo),
	0x0002: wrap(UnmarshalClientHashInfo, (*ASConn).onClientHashInfo),
	0x8009: wrap(UnmarshalUnknown8009, (*ASConn).onUnknown8009),
	0x800a: wrap(UnmarshalSetActiveCharacter, (*ASConn).onSetActiveCharacter),
	0x800d: wrap(UnmarshalDisconnect, (*ASConn).onDisconnect),
	0x800e: wrap(UnmarshalSetPlayerOnlineVisibilityStatus, (*ASConn).onSetPlayerOnlineVisibilityStatus),
	0x8016: wrap(UnmarshalLanguageInfo, (*ASConn).onLanguageInfo),
	0x801c: wrap(UnmarshalAddAccessKey, (*ASConn).onAddAccessKey),
	0x8020: wrap(UnmarshalUpdateSettings, (*ASConn).onUpdateSettings),
	0x8021: wrap(UnmarshalUpdateSettingsLength, (*ASConn).onUpdateSettingsLength),
	0x0023: wrap(UnmarshalUnknown8023, (*ASConn).on8023),
	0x8029: wrap(UnmarshalLoginCharacter, (*ASConn).onLoginCharacter),
	0x8035: wrap(UnmarshalAskServerResponse, (*ASConn).onAskServerResponse),
	0x8038: wrap(UnmarshalGetAccountInfo, (*ASConn).onGetAccountInfo),
	0x8007: wrap(UnmarshalDeleteCharacter, (*ASConn).onDeleteCharacter),
	0x0004: wrap(UnmarshalAccountLogin, (*ASConn).onAccountLogin),
	0x000f: wrap(UnmarshalUnknown000f, (*ASConn).onUnknown000f),
	0x0035: wrap(UnmarshalUnknown0035, (*ASConn).onUnknown0035),
	0x000e: wrap(UnmarshalUnknown000e, (*ASConn).onUnknown000e),
	0x0029: wrap(UnmarshalUnknown0029, (*ASConn).onUnknown0029),
}

func utf16leEncode(s string) []byte {
	runes := []rune(s)
	out := make([]byte, len(runes)*2)

	for i, r := range runes {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(r))
	}

	return out
}

func swapEndian(data []byte) []byte {
	if len(data)%4 != 0 {
		panic("data length must be a multiple of 4")
	}

	out := make([]byte, len(data))
	copy(out, data)

	for i := 0; i < len(out); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] =
			out[i+3], out[i+2], out[i+1], out[i]
	}

	return out
}

func calcPwHash(email, password string, salt []byte) []byte {
	// Stage 1: SHA1(UTF16LE(password + email))
	combined := utf16leEncode(password + email)

	h := sha1.New()
	h.Write(combined)
	stage1 := swapEndian(h.Sum(nil))

	// Stage 2: SHA1(salt + magic + stage1)
	stage2 := make([]byte, 0, len(salt)+4+len(stage1))
	stage2 = append(stage2, salt...)
	stage2 = append(stage2, 0x1D, 0xEA, 0xC6, 0x51)
	stage2 = append(stage2, stage1...)

	h = sha1.New()
	h.Write(stage2)

	return swapEndian(h.Sum(nil))
}

func (conn *ASConn) onAccountLogin(payload *AccountLogin) error {
	conn.log.Info().Str("email", payload.email).Int("req", payload.reqNumber).Msg("AttemptAccountLogin")
	testpass := "p"
	calculated := calcPwHash(payload.email, testpass, payload.salt)
	if !bytes.Equal(calculated, payload.passwordHash) {
		// wrong login
		conn.log.Info().Msg("bad login")
		conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 11))
		return nil
	}
	conn.log.Info().Msg("good login")
	//conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))

	// Re-retrieve account from DB:
	var ok bool
	conn.acc, ok = db.GetFullAccountByEmail("root@localhost")
	if !ok {
		// Account does not exist though a token was generated to connect to it?
		return fmt.Errorf("no such account during GetAccountInfo token verification")
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
	conn.EnqueuePacket(MarshalAccountBinaryInfo(payload.reqNumber, []byte{}))
	//conn.EnqueuePacket(MarshalAccountExtraInfoStart(payload.reqNumber, 0))
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
		conn.acc.UUID, lastCharUUID, 8, []byte{0x01, 0x00, 0x06, 0x00, 0x57, 0x00, 0x01, 0x00}, 23,
	))
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return nil
}

func (conn *ASConn) onUnknown000f(payload *Unknown000f) error {
	conn.log.Info().Msg("000f received")
	return nil
}
func (conn *ASConn) onUnknown0035(payload *Unknown0035) error {
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return nil
}
func (conn *ASConn) onUnknown000e(payload *Unknown000e) error {
	conn.log.Info().Int("unk", payload.unk).Msg("000e received")
	// Might be Play clicked
	return nil
}

func (conn *ASConn) onUnknown0029(payload *Unknown0029) error {
	conn.log.Info().Int("unk1", payload.unk1).Int("unk2", payload.unk2).Int("mapId", payload.mapId).Int("unk4", payload.unk4).Int("unk5", payload.unk5).Int("unk6", payload.unk6).Msg("0029 received")

	worldId := 0x10101010
	playerId := 0xbebafeca
	conn.EnqueuePacket(MarshalInstanceServerInfo(payload.unk1, worldId, 148, []byte{
		0x02, 0x00, // AF_INET
		0x17, 0xe0, // Port 6112
		0xc0, 0xa8, 0x01, 0x7c, // 192.168.1.124
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, playerId,
	))
	conn.EnqueuePacket(MarshalRequestResponse(payload.unk1, 0))
	return nil
}

func (conn *ASConn) onClientVersion(payload *ClientVersionInfo) error {
	conn.log.Info().
		Int("version", payload.clientVersion).
		Msg("ClientVersion")
	conn.state = StateReadClientSeed
	return nil
}

func (conn *ASConn) onClientSeed(payload *ClientSeed) error {
	rc4Key, publicBytes := crypt.GenerateEncryptionKey([64]byte(payload.seedBytes))

	conn.log.Info().Hex("seed", payload.seedBytes).Msg("ClientSeed")
	// Reply with ServerSeed, contents being the xored bytes.
	seedOut := MarshalServerSeed(publicBytes[:])
	conn.WritePacket(&seedOut)
	conn.state = StateCharacterScreen

	// Immediately after, enable RC4!
	var err error
	conn.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return fmt.Errorf("error creating rc4 encrypter: %s", err)
	}
	conn.dec, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return fmt.Errorf("error creating rc4 decrypter: %s", err)
	}

	return nil
}

func (conn *ASConn) onComputerInfo(payload *ComputerInfo) error {
	conn.log.Info().Str("user", payload.userName).Str("name", payload.computerName).Msg("ComputerUserInfo")
	return nil
}

func (conn *ASConn) onClientHashInfo(payload *ClientHashInfo) error {
	if payload.clientVersion != 25780 {
		// Wrong client version
		return fmt.Errorf("bad client version %d", payload.clientVersion)
	}
	conn.log.Debug().Int("clientVersion", payload.clientVersion).Msg("ClientHashInfo")

	conn.EnqueuePacket(MarshalSessionSaltInfo(0x51c6ea1d, 0xffffffff)) // Salt unknown
	return nil
}

func (conn *ASConn) on8000(payload *Unknown8000) error {
	return nil
}

func (conn *ASConn) on8023(payload *Unknown8023) error {
	//conn.EnqueuePacket(MarshalUnknown0000(0x8002e647, 0x17))
	return nil
}

func (conn *ASConn) onUnknown8009(payload *Unknown8009) error {
	conn.log.Info().Str("charName", payload.charName).Hex("unk", payload.unk3).Msg("Unknown8009")
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return nil
}

func (conn *ASConn) onGetAccountInfo(payload *GetAccountInfo) error {
	conn.log.Info().Hex("uuid1", payload.uuid1[:]).Hex("gameToken", payload.gameTokenFromPortalService[:]).Str("unkString", payload.unk1).Msg("GetAccountInfo")
	// Validate connection token
	tokenStr := db.UUIDStr(payload.gameTokenFromPortalService[:])
	accountId, ok := PortalService.ValidateConnectionToken(tokenStr)
	if !ok {
		// Bad connection token!
		return fmt.Errorf("invalid GameConnectionToken")
	}
	// Re-retrieve account from DB:
	conn.acc, ok = db.GetFullAccountByID(accountId)
	if !ok {
		// Account does not exist though a token was generated to connect to it?
		return fmt.Errorf("no such account during GetAccountInfo token verification")
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
		conn.acc.UUID, lastCharUUID, 3, []byte{0x01, 0x00, 0x06, 0x00}, 0x18,
	))
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))

	return nil
}

func (conn *ASConn) onAskServerResponse(payload *AskServerResponse) error {
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0xb5))
	return nil
}

func (conn *ASConn) onLanguageInfo(payload *LanguageInfo) error {
	conn.log.Debug().Int("languageCode", payload.languageCode).Msg("LanguageInfo")
	return nil
}

func (conn *ASConn) onDisconnect(payload *Disconnect) error {
	conn.log.Info().
		Int("errCode", payload.errorCode).Msg("Disconnect")
	conn.socket.Close()
	return nil
}

func (conn *ASConn) onLoginCharacter(payload *LoginCharacter) error {
	//if conn.state != StateCharacterScreen {
	//	return 0, fmt.Errorf("LoginCharacter: bad client state %v", conn.state)
	//}
	// For creating a new character, we get this:
	// 2:11PM INF LoginCharacter mapId=0 srv=auth unk2=11 unk4=0 unk5=4 unk6=0
	// For logging an existing character, we get this:
	// 2:12PM INF LoginCharacter mapId=165 srv=auth unk2=3 unk4=0 unk5=4 unk6=0
	conn.log.Debug().
		Int("unk2", payload.unk1).
		Int("mapId", payload.mapId).
		Int("unk4", payload.unk2).
		Int("unk5", payload.unk3).
		Int("unk6", payload.unk4).
		Msg("LoginCharacter")

	// TODO: we trust the mapId from client, whereas we ought to be using database value
	if payload.mapId == 0 {
		conn.log.Debug().Msg("State = StateCreateCharacter")
		conn.state = StateCreateCharacter
	} else {
		conn.state = StateInInstance
	}
	worldId := 0x10101010
	playerId := 0xbebafeca
	conn.EnqueuePacket(MarshalInstanceServerInfo(payload.reqNumber, worldId, payload.mapId, []byte{
		0x02, 0x00, // AF_INET
		0x17, 0xe0, // Port 6112
		0xc0, 0xa8, 0x01, 0x7c, // 192.168.1.124
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, playerId,
	))

	return nil
}

func (conn *ASConn) onAddAccessKey(payload *AddAccessKey) error {
	conn.log.Info().Str("key", payload.key).Msg("AddAccessKey")
	// 0 = OK
	// 102 = InvalidAccessKey
	// 103 = AccessKeyInUse
	// 105 = AccessKeyNotNeeded
	// 119 = AccessKeyAlreadyAppliedByYourAccount
	// 122 = AccessKeyDisabled
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return nil
}

func (conn *ASConn) onSetActiveCharacter(payload *SetActiveCharacter) error {
	conn.log.Debug().Str("charName", payload.charName).Msg("SetActiveCharacter")
	// Client sends this with empty charName if the active char was already selected upon a login char request
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return nil
}

func (conn *ASConn) onSetPlayerOnlineVisibilityStatus(payload *SetPlayerOnlineVisibilityStatus) error {
	conn.log.Debug().Int("newVisibility", payload.visibility).Msg("SetPlayerOnlineVisibilityStatus")
	return nil
}

func (conn *ASConn) onUpdateSettingsLength(payload *UpdateSettingsLength) error {
	conn.log.Debug().Int("unk2", payload.unk2).Msg("UpdateSettingsLength")
	conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	return nil
}

func (conn *ASConn) onUpdateSettings(payload *UpdateSettings) error {
	return nil
}

func (conn *ASConn) onDeleteCharacter(payload *DeleteCharacter) error {
	conn.log.Info().Str("name", payload.name).Msg("Delete Character")
	if err := db.DeleteDbChar(payload.name, conn.acc.ID); err != nil {
		conn.log.Error().Err(err).Msg("error during DeleteCharacter")
		conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 1))
	} else {
		conn.EnqueuePacket(MarshalRequestResponse(payload.reqNumber, 0))
	}
	return nil
}
