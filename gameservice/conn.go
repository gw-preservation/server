package GameService

import (
	"crypto/rc4"
	"fmt"
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

func (conn *GSConn) sendCreateCharacterInstanceInfo() {
	conn.log.Debug().Msg("sendCreateCharacterInstanceInfo")
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

	conn.EnqueuePacket(MarshalPlayerUpdateProfession(conn.player.agentId, 5, conn.player.secondaryProfession))

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

			conn.log.Debug().Msg("Transmitting item in slot!")
		}
	}

	conn.EnqueuePacket(MarshalItemWeaponSet(1))
	conn.EnqueuePacket(MarshalItemWeaponSet(2))
	conn.EnqueuePacket(MarshalItemWeaponSet(3))

	conn.EnqueuePacket(MarshalHeroInfo())
}

func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}

	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()
	conn.log.Debug().Str("opcode", fmt.Sprintf("%04x", op)).Msg("recv")

	if handler, ok := packetHandlers[op]; ok {
		consumed, err = handler(conn, &in)
	} else {
		consumed = len(data)
		conn.log.Warn().Str("op", fmt.Sprintf("%04x", op)).Hex("data", data).Msg("unhandled packet")
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
