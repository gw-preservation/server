package AuthService

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/crypt"
	"gw1/server/db"
	GameService "gw1/server/gameservice"
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
	0x8000: wrap(UnmarshalUnknown8000, (*ASConn).on8000),
	0x8001: wrap(UnmarshalComputerInfo, (*ASConn).onComputerInfo),
	0x8002: wrap(UnmarshalClientHashInfo, (*ASConn).onClientHashInfo),
	0x8009: wrap(UnmarshalSetCharacterName, (*ASConn).onSetCharacterName),
	0x800a: wrap(UnmarshalSetActiveCharacter, (*ASConn).onSetActiveCharacter),
	0x800d: wrap(UnmarshalDisconnect, (*ASConn).onDisconnect),
	0x800e: wrap(UnmarshalSetPlayerOnlineVisibilityStatus, (*ASConn).onSetPlayerOnlineVisibilityStatus),
	0x8016: wrap(UnmarshalLanguageInfo, (*ASConn).onLanguageInfo),
	0x801c: wrap(UnmarshalAddAccessKey, (*ASConn).onAddAccessKey),
	0x8020: wrap(UnmarshalUpdateSettings, (*ASConn).onUpdateSettings),
	0x8021: wrap(UnmarshalUpdateSettingsLength, (*ASConn).onUpdateSettingsLength),
	0x8023: wrap(UnmarshalUnknown8023, (*ASConn).on8023),
	0x8029: wrap(UnmarshalLoginCharacter, (*ASConn).onLoginCharacter),
	0x8035: wrap(UnmarshalAskServerResponse, (*ASConn).onAskServerResponse),
	0x8038: wrap(UnmarshalGetAccountInfo, (*ASConn).onGetAccountInfo),
	0x8007: wrap(UnmarshalDeleteCharacter, (*ASConn).onDeleteCharacter),
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

	return 66, nil
}

func (conn *ASConn) onComputerInfo(payload *ComputerInfo) error {
	conn.log.Info().Str("user", payload.userName).Str("name", payload.computerName).Msg("ComputerUserInfo")
	return nil
}

func (conn *ASConn) onClientHashInfo(payload *ClientHashInfo) error {
	if payload.clientVersion != 37600 {
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
	conn.EnqueuePacket(MarshalUnknown0000(0x8002e647, 0x17))
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
		conn.acc.UUID, lastCharUUID, 3, []byte{0x01, 0x00, 0x06, 0x00}, 0x18, 1,
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
	inst, err := GameService.InstanceManager.GetOrCreateInstanceByMapId(payload.mapId)
	if err != nil {
		conn.log.Error().Err(err).Msg("unable to create instance")
	}
	instanceTag := inst.GetTag()
	securityTag := GameService.GenerateConnectionTokenForInstance(instanceTag, conn.hasLoggedInThisSession)
	conn.EnqueuePacket(MarshalInstanceServerInfo(payload.reqNumber, int(instanceTag), payload.mapId, []byte{
		0x02, 0x00, // AF_INET
		0x17, 0xe0, // Port 6112
		0xc0, 0xa8, 0x01, 0x7c, // 192.168.1.124
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, int(securityTag),
	))
	conn.hasLoggedInThisSession = true

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

func (conn *ASConn) onSetCharacterName(payload *SetCharacterName) error {
	conn.log.Debug().Str("charName", payload.charName).Msg("SetCharacterName")
	// Client sends this with empty charName if the active char was already selected upon a login char request
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
