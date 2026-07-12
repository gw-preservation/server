package GameService

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/crypt"
	GwPacket "gw1/server/gwpacket"
	Item "gw1/server/item"
	"net"
	"time"

	"github.com/rs/zerolog"
)

type GSConn struct {
	socket *net.TCPConn
	enc    *rc4.Cipher
	dec    *rc4.Cipher
	out    GwPacket.Out
	closed bool
	log    zerolog.Logger
	player Player
}

func NewGSConn(socket *net.TCPConn, logCtx zerolog.Logger) *GSConn {
	conn := GSConn{
		socket: socket,
		closed: false,
		out:    GwPacket.NewOutRaw(),
		log:    logCtx.With().Str("srv", "game").Logger(),
	}
	conn.player = NewPlayer(&conn, logCtx)
	conn.log.Info().Msg("new client")
	go func() {
		for !conn.closed {
			time.Sleep(time.Millisecond * 20)
			if len(conn.out.GetBytes()) > 0 {
				conn.WritePacket(&conn.out)
				conn.out.Reset()
			}
		}
	}()
	return &conn
}

func (conn *GSConn) DecryptBytes(data []byte) {
	if conn.dec != nil {
		conn.dec.XORKeyStream(data, data)
	}
}

func (conn *GSConn) onVerifyClientConnection(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalVerifyClientConnection(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalVerifyClientConnection: %w", err)
	}
	conn.player.OnC2SVerifyConnection(payload)
	return pkt.Position(), nil
}
func (conn *GSConn) onClientSeed(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalClientSeed(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalClientSeed: %w", err)
	}
	rc4Key, publicBytes := crypt.GenerateEncryptionKey([64]byte(payload.seed))

	conn.dec, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 0, fmt.Errorf("error creating rc4 decrypter: %s", err)
	}
	resp := GwPacket.NewOutRaw()
	resp.Uint8(01)
	resp.Uint8(len(publicBytes) + 2)
	resp.Bytes(publicBytes[:])
	conn.WritePacket(&resp)

	conn.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return 0, fmt.Errorf("error creating rc4 encrypter: %s", err)
	}

	(*conn.player.connectedInstance).AddPlayer(&conn.player)

	return pkt.Position(), nil
}

func (conn *GSConn) onGPUInformation(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalGpuInformation(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalGPUInformation: %w", err)
	}

	conn.log.Info().Str("name", payload.driverName).Str("version", payload.driverVersion).Msg("GPUInfo")

	return pkt.Position(), nil
}

func (conn *GSConn) onDisconnect(pkt *GwPacket.In) (int, error) {
	conn.player.OnUserDisconnected()
	conn.Close()
	return pkt.Position(), nil
}

func (conn *GSConn) onInstanceLoadRequestStart(pkt *GwPacket.In) (int, error) {
	conn.log.Info().Msg("InstanceLoadRequestStart")
	return pkt.Position(), nil
}

func (conn *GSConn) onUpdateProfessionChoice(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalUpdateProfessionChoice(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalUpdateProfessionChoice: %w", err)
	}
	conn.player.OnC2SUpdateProfessionChoice(payload)
	// Now respond with updated items and profession

	return pkt.Position(), nil
}

func (conn *GSConn) onDyeEquipment(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalDyeEquipment(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalDyeEquipment: %w", err)
	}
	conn.player.OnC2SDyeEquipment(payload)

	return pkt.Position(), nil
}

func (conn *GSConn) sendCreateCharacterInstanceInfo() {
	conn.log.Warn().Msg("sendCreateCharacterInstanceInfo")
	conn.EnqueuePacket(MarshalInstancePlayerDataStart())
	itemStreamId := 1
	conn.EnqueuePacket(MarshalItemStreamCreate(itemStreamId))

	// Need at least one item so that the response to Dye change requests is accepted without crash

	conn.EnqueuePacket(MarshalItemGeneralInfo(
		1,
		2147595574,
		3,
		1,
		0,
		0,
		0,
		536875008,
		5,
		32,
		1,
		[]byte{0xa8, 0x21, 0x57, 0xd1, 0x8f, 0xb5, 0x6f, 0x16},
		[]uint32{608703488},
	))

	conn.EnqueuePacket(MarshalAgentUpdateAttributePoints(conn.player.agentId, 0, 0))

	conn.EnqueuePacket(MarshalPlayerUpdateProfession(conn.player.agentId, conn.player.primaryProfession, conn.player.secondaryProfession))

	conn.EnqueuePacket(MarshalAgentAttrUpdateInt(conn.player.agentId, 64, 0))

	conn.EnqueuePacket(MarshalInstancePlayerDataDone())
}

func (conn *GSConn) sendWorldInstanceHead() {

	conn.EnqueuePacket(MarshalInstancePlayerDataStart())

	conn.EnqueuePacket(MarshalInstanceLoadPlayerName(conn.player.name))
	conn.EnqueuePacket(MarshalInstanceLoadInfo(conn.player.playerId, int(conn.player.dbChar.LastOutpostID), false, 1, 0, false))
}

func (conn *GSConn) sendWorldInstanceBody() {
	resp := MarshalItemStreamCreate(1)
	conn.EnqueuePacket(resp)

	conn.EnqueuePacket(MarshalActivateWeaponSet(1))

	// Send bags:
	for bagIndex, bag := range conn.player.bags {
		// 1. Create bag
		if bag.Type == uint8(1) {
			// Inventory
			// Send the bag item itself now:
			backpack := Item.GetItemDefinitionById(32)
			conn.EnqueuePacket(MarshalItemGeneralInfo(
				1,
				int(backpack.ModelFileId),
				3,
				1,
				0,
				0,
				0,
				0x20001000,
				backpack.BaseMerchantValue,
				32,
				1,
				convertEncName(backpack.EncName),
				backpack.MarshalModifiers(),
			))
			conn.EnqueuePacket(MarshalItemUpdateName(1, conn.player.name))
			conn.EnqueuePacket(MarshalInventoryCreateBag(1, int(bag.Type), 0, bagIndex, int(bag.Capacity), 1))

		} else if bag.Type == uint8(2) {
			// Equipped
			conn.EnqueuePacket(MarshalItemUpdateName(1, conn.player.name))
			conn.EnqueuePacket(MarshalInventoryCreateBag(1, int(bag.Type), 21, bagIndex, int(bag.Capacity), 0))
		}

		// 2. Tell client about each item in that bag (GeneralInfo+Moved)
		for slotIndex, slot := range bag.Slots {
			if slot.ItemID == 0 || slot.ItemQuantity == 0 {
				continue
			}
			item := Item.GetItemDefinitionById(int(slot.ItemID))
			conn.EnqueuePacket(MarshalItemGeneralInfo(
				2+slotIndex,
				int(item.ModelFileId),
				int(slot.ItemType),
				0,
				8,
				0,
				0,
				0x22201000,
				item.BaseMerchantValue,
				int(slot.ItemID),
				1,
				convertEncName(item.EncName),
				item.MarshalModifiers(),
			))
			conn.EnqueuePacket(MarshalItemMovedToLocation(1, 2+slotIndex, bagIndex, slotIndex))

			conn.log.Info().Msg("Transmitting item in slot!")
		}
	}

	conn.EnqueuePacket(MarshalItemWeaponSet(1))
	conn.EnqueuePacket(MarshalItemWeaponSet(2))
	conn.EnqueuePacket(MarshalItemWeaponSet(3))

	conn.EnqueuePacket(MarshalHeroInfo())
}

func (conn *GSConn) onCreateCharRequestPlayer(pkt *GwPacket.In) (int, error) {
	conn.log.Info().Msg("CharCreationRequestPlayer")

	return pkt.Position(), nil
}

func (conn *GSConn) on8090(pkt *GwPacket.In) (int, error) {
	if pkt.Remaining() < 2 {
		return 0, nil
	}
	pkt.Skip(2)

	return pkt.Position(), nil
}

func (conn *GSConn) onInstanceLoadRequestSpawnPoint(pkt *GwPacket.In) (int, error) {
	conn.player.sendInstanceLoadSpawnPoint()
	return pkt.Position(), nil
}

func (conn *GSConn) onInstanceLoadRequestPlayers(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalInstanceLoadRequestPlayers(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalInstanceLoadRequestPlayers: %w", err)
	}
	conn.player.sendInstanceLoadRequestPlayers(payload)

	return pkt.Position(), nil
}

func (conn *GSConn) on8091(pkt *GwPacket.In) (int, error) {
	_, err := UnmarshalUnknown8091(pkt)
	if err != nil {
		return 0, fmt.Errorf("Unmarshal8091: %w", err)
	}

	return pkt.Position(), nil
}

func (conn *GSConn) onPingReply(pkt *GwPacket.In) (int, error) {
	_, err := UnmarshalPingReply(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalPingReply: %w", err)
	}
	resp := GwPacket.NewOut(0xd)
	resp.Uint32(1)
	conn.EnqueuePacket(resp)
	return pkt.Position(), nil
}

func (conn *GSConn) onChatMessage(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalChatMessage(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalChatMessage: %w", err)
	}
	conn.player.OnC2SChatMessage(payload)
	return in.Position(), nil
}

func (conn *GSConn) onCreateCharacterFinish(pkt *GwPacket.In) (int, error) {
	payload, err := UnmarshalCreateCharacterFinish(pkt)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalCreateCharacterFinish: %w", err)
	}
	appearance := ParseAppearanceBits(uint32(payload.appearance))

	conn.log.Info().Str("desiredName", payload.name).Interface("appearance", appearance).Msg("CreateCharacterFinish")

	// Simulate name taken:
	conn.EnqueuePacket(MarshalCharCreationError(29))

	// 0x187 is sent instead of 0x18A if name was successful

	return pkt.Position(), nil
}

func (conn *GSConn) onMoveToPoint(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalMoveToPoint(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalMoveToPoint: %w", err)
	}
	conn.player.connectedInstance.UpdateRequestedPlayerPos(&conn.player, payload.x, payload.y)
	conn.EnqueuePacket(MarshalMoveToPointS2C(conn.player.agentId, payload.x, payload.y, 0))
	return in.Position(), nil
}

func (conn *GSConn) onRotateAgent(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalRotateAgent(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalRotateAgent: %w", err)
	}
	conn.log.Debug().Int("unk1", payload.unk1).Int("unk2", payload.unk2).Msg("RotateAgent")
	return in.Position(), nil
}

func (conn *GSConn) onMovementUpdate(in *GwPacket.In) (int, error) {
	_, err := UnmarshalMovementUpdate(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalMovementUpdate %w", err)
	}
	return in.Position(), nil
}

func (conn *GSConn) onLastPosBeforeMoveCancelled(in *GwPacket.In) (int, error) {
	_, err := UnmarshalLastPosBeforeMoveCancelled(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalLastPosBeforeMoveCancelled %w", err)
	}
	return in.Position(), nil
}

func (conn *GSConn) onUpdateTarget(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalUpdateTarget(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalUnknown80c0: %w", err)
	}
	conn.log.Debug().Int("target", payload.targetAgentId).Str("playerName", conn.player.name).Msg("UpdateTarget")
	return in.Position(), nil
}

func (conn *GSConn) onInteractAgent(in *GwPacket.In) (int, error) {
	payload, err := UnmarshalInteractAgent(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalInteractAgent: %w", err)
	}
	conn.player.SendChatWarning(fmt.Sprintf("missing interaction definition for agent=%d,action=%d", payload.agentId, payload.action))
	conn.log.Info().Int("target", payload.agentId).Int("action", payload.action).Msg("InteractAgent")
	return in.Position(), nil
}

func (conn *GSConn) onCancelInteraction(in *GwPacket.In) (int, error) {
	_, err := UnmarshalCancelInteraction(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalCancelInteraction: %w", err)
	}
	return in.Position(), nil
}

func (conn *GSConn) Close() {
	conn.closed = true
	if conn.player.connectedInstance != nil {
		(*conn.player.connectedInstance).RemovePlayer(&conn.player)
	}
	conn.socket.Close()
}

func (conn *GSConn) onClientPingRequest(in *GwPacket.In) (int, error) {
	_, err := UnmarshalClientPingRequest(in)
	if err != nil {
		return 0, fmt.Errorf("UnmarshalClientPingRequest: %w", err)
	}
	return in.Position(), nil
}

func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}
	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()
	switch op {
	case 0x0500:
		consumed, err = conn.onVerifyClientConnection(&in)
	case 0x4200:
		consumed, err = conn.onClientSeed(&in)
	case 0x800a:
		consumed, err = conn.onGPUInformation(&in)
	case 0x800c:
		consumed, err = conn.onClientPingRequest(&in)
	case 0x8009:
		consumed, err = conn.onPingReply(&in)
	case 0x8027:
		consumed, err = conn.onCancelInteraction(&in)
	case 0x8038:
		consumed, err = conn.onInteractAgent(&in)
	case 0x803c:
		consumed, err = conn.onMovementUpdate(&in)
	case 0x803d:
		consumed, err = conn.onMoveToPoint(&in)
	case 0x803f:
		consumed, err = conn.onRotateAgent(&in)
	case 0x8046:
		consumed, err = conn.onLastPosBeforeMoveCancelled(&in)
	case 0x805f:
		consumed, err = conn.onUpdateProfessionChoice(&in)
	case 0x8063:
		consumed, err = conn.onChatMessage(&in)
	case 0x8083:
		consumed, err = conn.onDyeEquipment(&in)
	case 0x8087:
		consumed, err = conn.onInstanceLoadRequestSpawnPoint(&in)
	case 0x8088:
		consumed, err = conn.onCreateCharRequestPlayer(&in)
	case 0x808f:
		consumed, err = conn.onInstanceLoadRequestPlayers(&in)
	case 0x8089:
		consumed, err = conn.onInstanceLoadRequestStart(&in)
	case 0x808a:
		consumed, err = conn.onCreateCharacterFinish(&in)
	case 0x8090:
		consumed, err = conn.on8090(&in)
	case 0x8091:
		consumed, err = conn.on8091(&in)
	case 0x8008:
		consumed, err = conn.onDisconnect(&in)
	case 0x80c0:
		consumed, err = conn.onUpdateTarget(&in)
	default:
		consumed = len(data)
		conn.log.Warn().Str("op", fmt.Sprintf("%04x", op)).Hex("data", data).Msg("unhandled packet")
		// TEMPORARY HACK, REMOVE COMMENT AND HANDLE PACKETS PROPERLY!
	}
	if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}
	if err != nil {
		err = fmt.Errorf("HandleBytes(op=%04x): %w", op, err)
	}
	return consumed, err
}

func (conn *GSConn) Read(buf []byte) (int, error) {
	return conn.socket.Read(buf)
}

func (conn *GSConn) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	if conn.enc != nil {
		conn.enc.XORKeyStream(bts, bts)
	}
	_, err := conn.socket.Write(bts)
	return err
}

func (conn *GSConn) EnqueuePacket(packet GwPacket.Out) {
	conn.out.Merge(packet)
}
