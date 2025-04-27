package FileService

import (
	"fmt"
	GwPacket "gw1/server/gwpacket"
	"net"

	"github.com/rs/zerolog"
)

type Client struct {
	conn *net.TCPConn
	log  zerolog.Logger
}

func NewClient(conn *net.TCPConn, logCtx zerolog.Logger) *Client {
	fc := Client{
		conn: conn,
		log:  logCtx.With().Str("srv", "file").Logger(),
	}
	fc.log.Info().Msg("new client")
	return &fc
}

func (client *Client) onHelloMessage(in *GwPacket.In) (int, error) {
	if in.Remaining() < 3 {
		return 0, nil
	}
	in.Skip(3)
	return in.Position(), nil
}

func (client *Client) onInitialData(in *GwPacket.In) (int, error) {
	if in.Remaining() < 14 {
		return 0, nil
	}
	in.Skip(14)
	resp := GwPacket.NewOut(0x02f1)
	resp.Uint16(32)
	resp.Bytes([]byte{
		0xa6, 0xd0, 0x05, 0x00,
		0x26, 0xd2, 0x05, 0x00,
		0x9c, 0x43, 0x05, 0x00,
		0x31, 0xd2, 0x05, 0x00,
		0x32, 0xd2, 0x05, 0x00,
		0xda, 0xd0, 0x05, 0x00,
		0x30, 0xd2, 0x05, 0x00,
	})
	client.WritePacket(&resp)
	return in.Position(), nil
}

func (client *Client) onLoadingStatus(in *GwPacket.In) (int, error) {
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

func (client *Client) onHeartbeat(in *GwPacket.In) (int, error) {
	if in.Remaining() < 2 {
		return 0, nil
	}
	unk, err := in.Uint16()
	if err != nil {
		return 0, fmt.Errorf("read unk: %w", err)
	}
	resp := GwPacket.NewOut(0x09f1)
	resp.Uint16(unk)
	client.WritePacket(&resp)
	client.log.Info().Int("unk", unk).Msg("Heartbeat")

	return in.Position(), nil
}

func (client *Client) HandleBytes(data []byte) (int, error) {
	client.log.Info().Int("len", len(data)).Msg("HandleBytes")
	in := GwPacket.NewIn(data)
	op, err := in.Uint16()
	if err != nil {
		return 0, fmt.Errorf("read opcode: %w", err)
	}
	switch op {
	case 0x0001:
		return client.onHelloMessage(&in)
	case 0x00f1:
		return client.onInitialData(&in)
	case 0x10f1:
		return client.onLoadingStatus(&in)
	case 0x08f1:
		return client.onHeartbeat(&in)
	}
	client.log.Warn().Hex("data", data).Str("op", fmt.Sprintf("%04x", op)).Msg("unhandled message")
	return len(data), nil
}

func (client *Client) Read(buf []byte) (int, error) {
	return client.conn.Read(buf)
}

func (client *Client) Close() {
	client.conn.Close()
}

func (client *Client) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	_, err := client.conn.Write(bts)
	return err
}
