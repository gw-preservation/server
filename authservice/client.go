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

type Client struct {
	conn  *net.TCPConn
	state State
	enc   *rc4.Cipher
	dec   *rc4.Cipher
	out   GwPacket.Out
	log   zerolog.Logger
	acc   db.Account
}

func NewClient(conn *net.TCPConn, logCtx zerolog.Logger) *Client {
	client := Client{
		conn:  conn,
		state: StateReadClientVersion,
		log:   logCtx.With().Str("srv", "auth").Logger(),
		out:   GwPacket.NewOutRaw(),
	}
	client.log.Info().Msg("new client")
	return &client
}

func (client *Client) DecryptBytes(data []byte) {
	if client.dec != nil {
		client.dec.XORKeyStream(data, data)
	}
}

func (client *Client) HandleBytes(data []byte) (int, error) {
	inPkt := GwPacket.NewIn(data)
	if client.state == StateReadClientVersion {
		return client.onClientVersion(&inPkt)
	} else if client.state == StateReadClientSeed {
		return client.onClientSeed(&inPkt)
	} else {
		return client.onPacket(&inPkt)
	}
}

func (client *Client) onComputerInfo(in *GwPacket.In) (int, error) {
	payload, err := unmarshalComputerInfo(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalComputerInfo: %w", err)
	}

	client.log.Info().Str("user", payload.userName).Str("name", payload.computerName).Msg("ComputerUserInfo")
	return in.Position(), nil
}

func (client *Client) onClientHashInfo(in *GwPacket.In) (int, error) {
	payload, err := unmarshalClientHashInfo(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalClientHashInfo: %w", err)
	}
	if payload.clientVersion != 37550 {
		// Wrong client version
		client.log.Panic().Msgf("bad client version %d", payload.clientVersion)
	}
	client.log.Debug().
		Str("op", fmt.Sprintf("%04x", in.Opcode())).
		Int("clientVersion", payload.clientVersion).
		//Hex("unkHash", payload.unkHash[:]). // Maybe memory hash / hash of DH keys?
		Msg("")

	client.EnqueuePacket(newSessionInfo(0x51c6ea1d)) // Salt unknown
	return in.Position(), nil
}

func (client *Client) on8000(in *GwPacket.In) (int, error) {
	_, err := unmarshal8000(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshal8000: %w", err)
	}
	return in.Position(), nil
}

func (client *Client) on8023(in *GwPacket.In) (int, error) {
	_, err := unmarshal8023(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshal8023: %w", err)
	}
	resp := GwPacket.NewOut(0)
	resp.Uint32(0x8002e647) //TODO: What is this? It comes from [8000] packet
	resp.Bytes([]byte{0x17, 0x00, 0x00, 0x00})
	client.WritePacket(&resp)
	return in.Position(), nil
}

func (client *Client) on8038_GetAccountInfo(in *GwPacket.In) (int, error) {
	payload, err := unmarshalGetAccountInfo(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalGetAccountInfo: %w", err)
	}

	client.log.Info().Hex("uuid1", payload.uuid1[:]).Hex("gameToken", payload.gameTokenFromPortalService[:]).Str("unkString", payload.unkString).Msg("GetAccountInfo")
	// Validate connection token
	tokenStr := db.UUIDStr(payload.gameTokenFromPortalService[:])
	accountId, ok := PortalService.ValidateConnectionToken(tokenStr)
	if !ok {
		// Bad connection token!
		return 0, fmt.Errorf("invalid GameConnectionToken")
	}
	// Re-retrieve account from DB:
	client.acc, ok = db.GetFullAccountByID(accountId)
	if !ok {
		// Account does not exist though a token was generated to connect to it?
		return 0, fmt.Errorf("no such account during GetAccountInfo token verification")
	}

	// Now send all characters belonging to the account:
	lastCharUUID := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	for _, char := range client.acc.Characters {
		client.EnqueuePacket(newCharacterSummaryPacket(
			payload.reqNumber,
			char.Name,
			char.UUID,
			int(char.LastOutpostID),
			[8]byte(char.Appearance),
			char.EquipmentData,
		))
		lastCharUUID = char.UUID

		fmt.Printf("Btw, sent a char UUID of %s\n", db.UUIDStr(char.UUID))
	}
	client.EnqueuePacket(newAccountBinaryInfo_0016(payload.reqNumber))
	client.EnqueuePacket(newAccountExtraInfo_0014(payload.reqNumber, client.acc.UUID, lastCharUUID, 2, true))
	client.EnqueuePacket(newRequestResponse(payload.reqNumber, 0))

	return in.Position(), nil
}

func (client *Client) on8035_AskServerResponse(in *GwPacket.In) (int, error) {
	payload, err := unmarshalAskServerResponse(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalAskServerResponse: %w", err)
	}
	client.EnqueuePacket(newRequestResponse(payload.reqNumber, 0xb5))

	return in.Position(), nil
}

func (client *Client) on8016(in *GwPacket.In) (int, error) {
	payload, err := unmarshalLanguageInfo(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalLanguageInfo: %w", err)
	}
	client.log.Info().
		Int("languageCode", payload.languageId).
		Msg("LanguageUpdate")
	return in.Position(), nil
}

func (client *Client) on800d_Disconnect(in *GwPacket.In) (int, error) {
	payload, err := unmarshalDisconnect(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalDisconnect: %w", err)
	}
	client.log.Info().
		Int("errCode", payload.errorCode).Msg("Disconnect")
	client.conn.Close()
	return in.Position(), nil
}

func (client *Client) on8029_LoginCharacter(in *GwPacket.In) (int, error) {
	payload, err := unmarshalLoginCharacter(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalLoginCharacter: %w", err)
	}
	//if client.state != StateCharacterScreen {
	//	return 0, fmt.Errorf("LoginCharacter: bad client state %v", client.state)
	//}
	// For creating a new character, we get this:
	// 2:11PM INF LoginCharacter mapId=0 srv=auth unk2=11 unk4=0 unk5=4 unk6=0
	// For logging an existing character, we get this:
	// 2:12PM INF LoginCharacter mapId=165 srv=auth unk2=3 unk4=0 unk5=4 unk6=0
	client.log.Info().
		Int("unk2", payload.unk2).
		Int("mapId", payload.mapId).
		Int("unk4", payload.unk4).
		Int("unk5", payload.unk5).
		Int("unk6", payload.unk6).
		Msg("LoginCharacter")

	if payload.mapId == 0 {
		client.log.Info().Msg("State = StateCreateCharacter")
		client.state = StateCreateCharacter
	} else {
		client.state = StateInInstance
	}
	worldId := 0x10101010
	playerId := 0xbebafeca
	client.EnqueuePacket(newInstanceServerInfo(payload.reqNumber, worldId, payload.mapId, playerId))

	return in.Position(), nil
}

func (client *Client) on801c_AddAccessKey(in *GwPacket.In) (int, error) {
	payload, err := unmarshalAddAccessKey(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalAddAccessKey: %w", err)
	}
	client.log.Info().Str("key", payload.accessKey).Msg("AddAccessKey")
	client.EnqueuePacket(newRequestResponse(payload.reqNumber, 0))
	return in.Position(), nil
}

func (client *Client) on800a_SetActiveCharacter(in *GwPacket.In) (int, error) {
	payload, err := unmarshalSetActiveCharacter(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalSetActiveCharacter: %w", err)
	}
	client.log.Info().Str("charName", payload.charName).Msg("SetActiveCharacter")
	// Client sends this with empty charName if the active char was already selected upon a login char request

	/*if payload.charName == "Char B" {
		client.EnqueuePacket(newAccountExtraInfo_0014(payload.reqNumber, char2UUID[:], 4, true))
	} else {
		client.EnqueuePacket(newAccountExtraInfo_0014(payload.reqNumber, char1UUID[:], 4, true))
	}*/
	client.EnqueuePacket(newRequestResponse(payload.reqNumber, 0))
	return in.Position(), nil
}

func (client *Client) on800e_SetPlayerOnlineVisibilityStatus(in *GwPacket.In) (int, error) {
	payload, err := unmarshalSetPlayerOnlineVisibilityStatus(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalSetPlayerOnlineVisibilityStatus: %w", err)
	}
	client.log.Info().Int("newVisibility", payload.newVisibility).Msg("SetPlayerOnlineVisibilityStatus")
	return 6, nil
}

func (client *Client) onPacket(in *GwPacket.In) (consumed int, err error) {
	op, _ := in.Uint16()
	switch op {
	case 0x8001:
		consumed, err = client.onComputerInfo(in)
	case 0x8002:
		consumed, err = client.onClientHashInfo(in)
	case 0x8000:
		consumed, err = client.on8000(in)
	case 0x800a:
		consumed, err = client.on800a_SetActiveCharacter(in)
	case 0x8016:
		consumed, err = client.on8016(in)
	case 0x801c:
		consumed, err = client.on801c_AddAccessKey(in)
	case 0x8023:
		consumed, err = client.on8023(in)
	case 0x8029:
		consumed, err = client.on8029_LoginCharacter(in)
	case 0x8038:
		consumed, err = client.on8038_GetAccountInfo(in)
	case 0x8035:
		consumed, err = client.on8035_AskServerResponse(in)
	case 0x800e:
		consumed, err = client.on800e_SetPlayerOnlineVisibilityStatus(in)
	case 0x800d: //, 0x800e:
		consumed, err = client.on800d_Disconnect(in)
	default:
		// Unexpected opcode!
		return 0, fmt.Errorf("[%04x] UNEXPECTED; len=%d", op, in.Remaining())
	}
	client.log.Debug().Str("op", fmt.Sprintf("%04x", op)).Int("consumed", consumed).Int("remaining", in.Remaining()).Msg("")
	if len(client.out.GetBytes()) > 0 {
		client.WritePacket(&client.out)
		client.out.Reset()
	}
	return
}

func (client *Client) onClientVersion(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalClientVersionInfo(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalClientVersionPacket: %w", err)
	}
	client.log.Info().
		Int("version", payload.clientVersion).
		Msg("ClientVersion")
	client.state = StateReadClientSeed
	return 16, nil
}

func (client *Client) onClientSeed(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalClientSeed(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalClientSeed")
	}
	rc4Key, publicBytes := crypt.GenerateEncryptionKey(payload.seedBytes)

	// Reply with ServerSeed, contents being the xored bytes.
	client.WritePacket(newServerSeed(publicBytes[:]))
	client.state = StateCharacterScreen

	// Immediately after, enable RC4!
	client.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 66, fmt.Errorf("error creating rc4 encrypter: %s", err)
	}
	client.dec, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 66, fmt.Errorf("error creating rc4 decrypter: %s", err)
	}

	client.log.Info().Msg("ClientSeed")

	return 66, nil
}

func (client *Client) Read(buf []byte) (int, error) {
	return client.conn.Read(buf)
}

func (client *Client) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	if client.enc != nil {
		client.enc.XORKeyStream(bts, bts)
	}
	_, err := client.conn.Write(bts)
	return err
}

func (client *Client) Close() {
	client.conn.Close()
}

func (client *Client) EnqueuePacket(packet GwPacket.Out) {
	client.out.Merge(packet)
}
