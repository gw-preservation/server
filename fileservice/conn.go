package FileService

import (
	"fmt"
	GwPacket "gw1/server/gwpacket"
	"net"

	"github.com/rs/zerolog"
)

type FSConn struct {
	socket *net.TCPConn
	log    zerolog.Logger
}

func NewFSConn(socket *net.TCPConn, logCtx zerolog.Logger) *FSConn {
	fc := FSConn{
		socket: socket,
		log:    logCtx.With().Str("srv", "file").Logger(),
	}
	fc.log.Info().Msg("new client")
	return &fc
}

func (conn *FSConn) onHelloMessage(in *GwPacket.In) (int, error) {
	if in.Remaining() < 3 {
		return 0, nil
	}
	in.Skip(3)
	return in.Position(), nil
}

/**
37550:
0xa6, 0xd0, 0x05, 0x00,
0x26, 0xd2, 0x05, 0x00,
0x9c, 0x43, 0x05, 0x00,
0x31, 0xd2, 0x05, 0x00,
0x32, 0xd2, 0x05, 0x00,
0xda, 0xd0, 0x05, 0x00,
0x30, 0xd2, 0x05, 0x00,

37578:
0xa6, 0xd0, 0x05, 0x00,
0xc3, 0xd2, 0x05, 0x00,
0x9c, 0x43, 0x05, 0x00,
0xc5, 0xd2, 0x05, 0x00,
0xc6, 0xd2, 0x05, 0x00,
0xda, 0xd0, 0x05, 0x00,
0xc4, 0xd2, 0x05, 0x00,

37587:
0xa6, 0xd0, 0x05, 0x00,
0xc3, 0xd2, 0x05, 0x00,
0x9c, 0x43, 0x05, 0x00,
0xc8, 0xd2, 0x05, 0x00,
0xc9, 0xd2, 0x05, 0x00,
0xda, 0xd0, 0x05, 0x00,
0xc7, 0xd2, 0x05, 0x00,
*/

func (conn *FSConn) onInitialData(in *GwPacket.In) (int, error) {
	if in.Remaining() < 14 {
		return 0, nil
	}
	in.Skip(14)
	resp := GwPacket.NewOut(0x02f1)
	resp.Uint16(32)
	resp.Bytes([]byte{
		0xa6, 0xd0, 0x05, 0x00,
		0xc3, 0xd2, 0x05, 0x00,
		0x9c, 0x43, 0x05, 0x00,
		0xc8, 0xd2, 0x05, 0x00,
		0xc9, 0xd2, 0x05, 0x00,
		0xda, 0xd0, 0x05, 0x00,
		0xc7, 0xd2, 0x05, 0x00, // fileId for Gw.exe
	})
	conn.WritePacket(&resp)
	return in.Position(), nil
}

func (conn *FSConn) onLoadingStatus(in *GwPacket.In) (int, error) {
	if in.Remaining() < 2 {
		return 0, nil
	}
	len, err := in.Uint16()
	if err != nil {
		return 0, fmt.Errorf("read len: %w", err)
	}
	len -= 4
	if in.Remaining() < len {
		return 0, nil
	}
	in.Skip(len)
	return in.Position(), nil
}

func (conn *FSConn) onHeartbeat(in *GwPacket.In) (int, error) {
	if in.Remaining() < 2 {
		return 0, nil
	}
	unk, err := in.Uint16()
	if err != nil {
		return 0, fmt.Errorf("read unk: %w", err)
	}
	resp := GwPacket.NewOut(0x09f1)
	resp.Uint16(unk)
	conn.WritePacket(&resp)
	conn.log.Info().Int("unk", unk).Msg("Heartbeat")

	return in.Position(), nil
}

func (conn *FSConn) HandleBytes(data []byte) (int, error) {
	conn.log.Info().Int("len", len(data)).Msg("HandleBytes")
	in := GwPacket.NewIn(data)
	op, err := in.Uint16()
	if err != nil {
		return 0, fmt.Errorf("read opcode: %w", err)
	}
	switch op {
	case 0x0001:
		return conn.onHelloMessage(&in)
	case 0x00f1:
		return conn.onInitialData(&in)
	case 0x10f1:
		return conn.onLoadingStatus(&in)
	case 0x08f1:
		return conn.onHeartbeat(&in)
	}
	conn.log.Warn().Hex("data", data).Str("op", fmt.Sprintf("%04x", op)).Msg("unhandled message")
	return len(data), nil
}

func (conn *FSConn) Read(buf []byte) (int, error) {
	return conn.socket.Read(buf)
}

func (conn *FSConn) Close() {
	conn.socket.Close()
}

func (conn *FSConn) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	_, err := conn.socket.Write(bts)
	return err
}
