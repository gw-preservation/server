package GameService

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/crypt"
	GwPacket "gw1/server/gwpacket"
	"net"
	"time"

	"github.com/rs/zerolog"
)

type Client struct {
	conn   *net.TCPConn
	enc    *rc4.Cipher
	dec    *rc4.Cipher
	out    GwPacket.Out
	mapId  int
	closed bool
	log    zerolog.Logger
	player Player
}

func NewClient(conn *net.TCPConn, logCtx zerolog.Logger) *Client {
	client := Client{
		conn:   conn,
		closed: false,
		out:    GwPacket.NewOutRaw(),
		log:    logCtx.With().Str("srv", "game").Logger(),
		mapId:  0,
	}
	client.player = NewPlayer(&client, logCtx)
	client.log.Info().Msg("new client")
	go func() {
		for !client.closed {
			time.Sleep(time.Millisecond * 20)
			if len(client.out.GetBytes()) > 0 {
				client.WritePacket(&client.out)
				client.out.Reset()
			}
		}
	}()
	return &client
}

func (client *Client) DecryptBytes(data []byte) {
	if client.dec != nil {
		client.dec.XORKeyStream(data, data)
	}
}

func (client *Client) onVerifyClientConnection(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalVerifyClientConnection(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalverifyClientConnection: %w", err)
	}
	// we can validate here, or ignore while developing locally
	client.mapId = payload.sharedVal1

	// Hook client up to an instance
	inst := InstanceManager.GetOrCreateInstanceByMapId(client.mapId)
	if inst == nil {
		// something went wrong - decline connection
		// TODO: decline connection
		panic("unimplemented: decline connection due to instance creation error")
	}
	client.player.connectedInstance = inst

	client.log.Info().Int("mapId", client.mapId).Msg("VerifyClientConnection")
	return pkt.Position(), nil
}
func (client *Client) onClientSeed(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalClientSeed(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalClientSeed: %w", err)
	}
	client.log.Info().Msg("ClientSeed")
	rc4Key, publicBytes := crypt.GenerateEncryptionKey(payload.seed)

	client.dec, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 0, fmt.Errorf("error creating rc4 decrypter: %s", err)
	}
	resp := GwPacket.NewOutRaw()
	resp.Uint8(01)
	resp.Uint8(len(publicBytes) + 2)
	resp.Bytes(publicBytes[:])
	client.WritePacket(&resp)

	client.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 0, fmt.Errorf("error creating rc4 encrypter: %s", err)
	}

	(*client.player.connectedInstance).AddPlayer(&client.player)

	return pkt.Position(), nil
}

func (client *Client) onGPUInformation(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalGPUInformation(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalGPUInformation: %w", err)
	}

	client.log.Info().Str("name", payload.driverName).Str("version", payload.driverVersion).Msg("GPUInfo")

	return pkt.Position(), nil
}

func (client *Client) onDisconnect(pkt *GwPacket.In) (int, error) {
	/*
		CtoS packet(0x8) {
		} endpacket(0x8)
	*/
	client.player.OnUserDisconnected()
	client.Close()
	return pkt.Position(), nil
}

func (client *Client) onInstanceLoadRequestStart(pkt *GwPacket.In) (int, error) {
	/*
		CtoS packet(0x89) {
		} endpacket(0x89)
	*/
	client.log.Info().Msg("InstanceLoadRequestStart")

	return pkt.Position(), nil
}

func (client *Client) onUpdateProfessionChoice(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalUpdateProfessionChoice(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalUpdateProfessionChoice: %w", err)
	}
	client.player.OnC2SUpdateProfessionChoice(payload)
	// Now respond with updated items and profession

	return pkt.Position(), nil
}

func (client *Client) onDyeEquipment(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalDyeEquipment(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalDyeEquipment: %w", err)
	}
	client.player.OnC2SDyeEquipment(payload)

	return pkt.Position(), nil
}

func (client *Client) sendCreateCharacterInstanceInfo() {
	client.log.Warn().Msg("sendCreateCharacterInstanceInfo")
	client.EnqueuePacket(newInstancePlayerDataStart())
	itemStreamId := 1
	client.EnqueuePacket(newItemStreamCreate(itemStreamId))

	// Need at least one item so that the response to Dye change requests is accepted without crash
	client.EnqueuePacket(newItemGeneralInfo(itemGeneralInfo{
		itemLocalId:   1,
		fileId:        2147595574,
		itemType:      3,
		unk1:          1,
		itemFlags:     536875008,
		merchantPrice: 5,
		itemId:        32,
		quantity:      1,
		encNameBytes:  []byte{0xa8, 0x21, 0x57, 0xd1, 0x8f, 0xb5, 0x6f, 0x16},
		unk3:          608703488,
	}))

	client.EnqueuePacket(newAgentUpdateAttributePoints(1, 0, 0))

	client.EnqueuePacket(newPlayerUpdateProfession(1, 1, 0))

	client.EnqueuePacket(newAgentAttrUpdateInt(1, 64, 0))

	client.EnqueuePacket(newInstancePlayerDataDone())
}

func (client *Client) sendWorldInstanceHead() {

	client.EnqueuePacket(newInstancePlayerDataStart())

	client.EnqueuePacket(newInstanceLoadPlayerName("Scout Char"))

	client.EnqueuePacket(newInstanceLoadInfo(1, client.mapId, false, 1, 0, false))
}

func (client *Client) sendWorldInstanceBody() {
	resp := newItemStreamCreate(1)
	client.EnqueuePacket(resp)

	client.EnqueuePacket(newActivateWeaponSet(1))

	client.EnqueuePacket(newInventoryCreateBag(1, 1, 0, 2, 20, 8))
	client.EnqueuePacket(newInventoryCreateBag(1, 2, 21, 3, 9, 0))
	// skipping lots of 160/other item opcodes
	client.EnqueuePacket(newInventoryCreateBag(1, 3, 6, 4, 12, 0))
	client.EnqueuePacket(newInventoryCreateBag(1, 4, 7, 5, 25, 0))
	client.EnqueuePacket(newInventoryCreateBag(1, 4, 8, 6, 25, 0))
	client.EnqueuePacket(newInventoryCreateBag(1, 4, 9, 7, 25, 0))
	client.EnqueuePacket(newInventoryCreateBag(1, 4, 10, 8, 25, 0))
	client.EnqueuePacket(newInventoryCreateBag(1, 4, 11, 9, 25, 0))
	client.EnqueuePacket(newInventoryCreateBag(1, 5, 5, 10, 41, 0))

	client.EnqueuePacket(newItemWeaponSet(1))
	client.EnqueuePacket(newItemWeaponSet(2))
	client.EnqueuePacket(newItemWeaponSet(3))

	client.EnqueuePacket(newHeroInfo())
}

func (client *Client) onCreateCharRequestPlayer(pkt *GwPacket.In) (int, error) {
	/*
		CtoS packet(0x88) {
		} endpacket(0x88)
	*/
	client.log.Info().Msg("CharCreationRequestPlayer")

	return pkt.Position(), nil
}

func (client *Client) on8090(pkt *GwPacket.In) (int, error) {
	/*
		CtoS packet(0x90) {
		} endpacket(0x90)
	*/
	if pkt.Remaining() < 2 {
		return 0, nil
	}
	pkt.Skip(2)
	client.log.Info().Msg("8090")

	return pkt.Position(), nil
}

func (client *Client) onInstanceLoadRequestSpawnPoint(pkt *GwPacket.In) (int, error) {
	client.player.sendInstanceLoadSpawnPoint()
	return pkt.Position(), nil
}

func (client *Client) onInstanceLoadRequestSync(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalInstanceLoadRequestSync(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalInstanceLoadRequestSync: %w", err)
	}
	client.player.sendInstanceLoadSync(payload)

	return pkt.Position(), nil
}

func (client *Client) on8091(pkt *GwPacket.In) (int, error) {
	_, err := unmarshal8091(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshal8091: %w", err)
	}

	return pkt.Position(), nil
}

func (client *Client) onPingReply(pkt *GwPacket.In) (int, error) {
	_, err := unmarshalPingReply(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalPingReply: %w", err)
	}
	resp := GwPacket.NewOut(0xd)
	resp.Uint32(1)
	client.EnqueuePacket(resp)
	return pkt.Position(), nil
}

func (client *Client) onChatMessage(in *GwPacket.In) (int, error) {
	payload, err := unmarshalChatMessage(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalChatMessage: %w", err)
	}
	client.player.OnC2SChatMessage(payload)
	return in.Position(), nil
}

func (client *Client) onCreateCharacterFinish(pkt *GwPacket.In) (int, error) {
	payload, err := unmarshalCreateCharacterFinish(pkt)
	if err != nil {
		return 0, fmt.Errorf("unmarshalCreateCharacterFinish: %w", err)
	}
	client.log.Info().Str("desiredName", payload.charName).Hex("custom", payload.appearanceBits[:]).Msg("CreateCharacterFinish")

	// Simulate name taken:
	client.EnqueuePacket(newCharCreationError(29))

	// 0x187 is sent instead of 0x18A if name was successful

	return pkt.Position(), nil
}

func (client *Client) onMoveToPoint(in *GwPacket.In) (int, error) {
	payload, err := unmarshalMoveToPoint(in)
	if err != nil {
		return 0, fmt.Errorf("unmarshalMoveToPoint: %w", err)
	}
	client.log.Info().Float32("x", payload.x).Float32("y", payload.y).Msg("MoveToPoint")
	client.EnqueuePacket(newMoveToPoint(2, payload.x, payload.y))
	return in.Position(), nil
}

func (client *Client) Close() {
	client.closed = true
	if client.player.connectedInstance != nil {
		(*client.player.connectedInstance).RemovePlayer(&client.player)
	}
	client.conn.Close()
}

func (client *Client) HandleBytes(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}
	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()
	switch op {
	case 0x0500:
		consumed, err = client.onVerifyClientConnection(&in)
	case 0x4200:
		consumed, err = client.onClientSeed(&in)
	case 0x800a:
		consumed, err = client.onGPUInformation(&in)
	case 0x8009:
		consumed, err = client.onPingReply(&in)
	case 0x803d:
		consumed, err = client.onMoveToPoint(&in)
	case 0x805f:
		consumed, err = client.onUpdateProfessionChoice(&in)
	case 0x8063:
		consumed, err = client.onChatMessage(&in)
	case 0x8083:
		consumed, err = client.onDyeEquipment(&in)
	case 0x8087:
		consumed, err = client.onInstanceLoadRequestSpawnPoint(&in)
	case 0x8088:
		consumed, err = client.onCreateCharRequestPlayer(&in)
	case 0x808f:
		consumed, err = client.onInstanceLoadRequestSync(&in)
	case 0x8089:
		consumed, err = client.onInstanceLoadRequestStart(&in)
	case 0x808a:
		consumed, err = client.onCreateCharacterFinish(&in)
	case 0x8090:
		consumed, err = client.on8090(&in)
	case 0x8091:
		consumed, err = client.on8091(&in)
	case 0x8008:
		consumed, err = client.onDisconnect(&in)
	default:
		consumed = len(data)
		client.log.Warn().Str("op", fmt.Sprintf("%04x", op)).Msg("unhandled packet")
		// TEMPORARY HACK, REMOVE COMMENT AND HANDLE PACKETS PROPERLY!
		//err = fmt.Errorf("unhandled packet with len %d", in.Remaining())
	}
	client.log.Debug().Str("op", fmt.Sprintf("%04x", op)).Int("consumed", consumed).Int("remaining", in.Remaining()).Int("sent", len(client.out.GetBytes())).Msg("")
	if len(client.out.GetBytes()) > 0 {
		client.WritePacket(&client.out)
		client.out.Reset()
	}
	if err != nil {
		err = fmt.Errorf("HandleBytes(op=%04x): %w", op, err)
	}
	return consumed, err
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

func (client *Client) EnqueuePacket(packet GwPacket.Out) {
	client.out.Merge(packet)
}
