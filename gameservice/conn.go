package GameService

import (
	"crypto/rc4"
	"fmt"
	GwPacket "gw1/server/gwpacket"
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
			time.Sleep(time.Millisecond * 50)
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
	conn.EnqueuePacket(MarshalItemStreamCreate(1))

	// Need at least one item so that the response to Dye change requests is accepted without crash
	conn.player.itemMgr.AddBag(20, 1) // backpack
	conn.player.itemMgr.AddBag(9, 2)  // equipments

	conn.EnqueuePacket(MarshalAgentUpdateAttributePoints(conn.player.agentId, 0, 0))
	conn.EnqueuePacket(MarshalPlayerUpdateProfession(conn.player.agentId, 1, 0))
	conn.EnqueuePacket(MarshalAgentAttrUpdateInt(64, conn.player.agentId, 0))

	conn.EnqueuePacket(MarshalInstancePlayerDataDone())
}

func (conn *GSConn) sendWorldInstanceHead() {

	conn.EnqueuePacket(MarshalInstancePlayerDataStart())

	conn.EnqueuePacket(MarshalInstanceLoadPlayerName(conn.player.name))
	conn.EnqueuePacket(MarshalInstanceLoadInfo(conn.player.playerId, conn.player.connectedInstance.mapId, conn.player.connectedInstance.IsExplorable(), 1, 0, false))
}

func (conn *GSConn) sendWorldInstanceBody() {
	itemStreamId := 1
	resp := MarshalItemStreamCreate(itemStreamId)
	conn.EnqueuePacket(resp)

	conn.EnqueuePacket(MarshalActivateWeaponSet(itemStreamId))

	conn.player.TransmitItems()

	conn.EnqueuePacket(MarshalItemWeaponSet(itemStreamId, 1))
	conn.EnqueuePacket(MarshalItemWeaponSet(itemStreamId, 2))
	conn.EnqueuePacket(MarshalItemWeaponSet(itemStreamId, 3))

	conn.EnqueuePacket(MarshalHeroInfo())
}

func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}

	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()
	//conn.log.Debug().Str("opcode", fmt.Sprintf("%04x", op)).Msg("recv")

	if handler, ok := packetHandlers[op]; ok {
		consumed, err = handler(conn, &in)
	} else {
		consumed = len(data)
		conn.log.Warn().Str("op", fmt.Sprintf("%04x", op)).Hex("data", data).Msg("unhandled packet")
	}

	/*if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}*/

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
