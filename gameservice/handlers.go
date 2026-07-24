package GameService

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/crypt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
)

type packetHandler func(*GSConn, *GwPacket.In) (int, error)

func wrap[T any](
	unmarshal func(*GwPacket.In) (T, error),
	handler func(*GSConn, *T) error,
) packetHandler {
	return func(conn *GSConn, in *GwPacket.In) (int, error) {
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
	0x0500: wrap(UnmarshalVerifyClientConnection, (*GSConn).onVerifyClientConnection),
	0x4200: wrap(UnmarshalClientSeed, (*GSConn).onClientSeed),
	0x8008: wrap(UnmarshalClientDisconnect, (*GSConn).onClientDisconnect),
	0x8009: wrap(UnmarshalPingReply, (*GSConn).onPingReply),
	0x800a: wrap(UnmarshalGpuInformation, (*GSConn).onGPUInformation),
	0x800c: wrap(UnmarshalClientPingRequest, (*GSConn).onClientPingRequest),
	0x8027: wrap(UnmarshalCancelInteraction, (*GSConn).onCancelInteraction),
	0x8038: wrap(UnmarshalInteractAgent, (*GSConn).onInteractAgent),
	0x803c: wrap(UnmarshalMovementUpdate, (*GSConn).onMovementUpdate),
	0x803d: wrap(UnmarshalMoveToPoint, (*GSConn).onMoveToPoint),
	0x803f: wrap(UnmarshalRotateAgent, (*GSConn).onRotateAgent),
	0x8046: wrap(UnmarshalLastPosBeforeMoveCancelled, (*GSConn).onLastPosBeforeMoveCancelled),
	0x805f: wrap(UnmarshalUpdateProfessionChoice, (*GSConn).onUpdateProfessionChoice),
	0x8063: wrap(UnmarshalChatMessage, (*GSConn).onChatMessage),
	0x8083: wrap(UnmarshalDyeEquipment, (*GSConn).onDyeEquipment),
	0x8087: wrap(UnmarshalInstanceLoadRequestSpawnPoint, (*GSConn).onInstanceLoadRequestSpawnPoint),
	0x8088: wrap(UnmarshalCreateCharRequestPlayer, (*GSConn).onCreateCharRequestPlayer),
	0x8089: wrap(UnmarshalInstanceLoadRequestStart, (*GSConn).onInstanceLoadRequestStart),
	0x808a: wrap(UnmarshalCreateCharacterFinish, (*GSConn).onCreateCharacterFinish),
	0x808f: wrap(UnmarshalInstanceLoadRequestPlayers, (*GSConn).onInstanceLoadRequestPlayers),
	0x8090: wrap(UnmarshalUnknown8090, (*GSConn).on8090),
	0x8091: wrap(UnmarshalUnknown8091, (*GSConn).on8091),
	0x80c0: wrap(UnmarshalUpdateTarget, (*GSConn).onUpdateTarget),
	0x80b0: wrap(UnmarshalMapTravelToOutpost, (*GSConn).onMapTravelToOutpost),
}

func (conn *GSConn) onCreateCharRequestPlayer(payload *CreateCharRequestPlayer) error {
	return nil
}

func (conn *GSConn) on8090(payload *Unknown8090) error {
	return nil
}

func (conn *GSConn) onInstanceLoadRequestSpawnPoint(payload *InstanceLoadRequestSpawnPoint) error {
	conn.player.sendInstanceLoadSpawnPoint()
	return nil
}

func (conn *GSConn) onInstanceLoadRequestPlayers(payload *InstanceLoadRequestPlayers) error {
	conn.player.sendInstanceLoadRequestPlayers(*payload)
	return nil
}

func (conn *GSConn) on8091(payload *Unknown8091) error {
	return nil
}

func (conn *GSConn) onPingReply(payload *PingReply) error {
	resp := GwPacket.NewOut(0xd)
	resp.Uint32(1)
	conn.EnqueuePacket(resp)
	return nil
}

func (conn *GSConn) onChatMessage(payload *ChatMessage) error {
	conn.player.OnC2SChatMessage(*payload)
	return nil
}

func (conn *GSConn) onCreateCharacterFinish(payload *CreateCharacterFinish) error {
	appearance := ParseAppearanceBits(uint32(payload.appearance))
	conn.log.Info().Str("desiredName", payload.name).Interface("appearance", appearance).Msg("CreateCharacterFinish")

	if db.CharacterNameExists(payload.name) {
		conn.EnqueuePacket(MarshalCharCreationError(29))
		return nil
	}
	char := db.AddDbChar(conn.player.dbAcc.ID, payload.name, int(appearance.PrimaryProfession), uint32(payload.appearance))

	varbs := []byte{}
	conn.EnqueuePacket(MarshalCharCreationFinish(char.UUID, payload.name, 148, varbs))

	return nil
}

func (conn *GSConn) onMoveToPoint(payload *MoveToPoint) error {
	conn.player.connectedInstance.UpdateRequestedPlayerPos(&conn.player, payload.x, payload.y)
	conn.EnqueuePacket(MarshalMoveToPointS2C(conn.player.agentId, payload.x, payload.y, 0))
	return nil
}

func (conn *GSConn) onRotateAgent(payload *RotateAgent) error {
	conn.log.Debug().Int("unk1", payload.unk1).Int("unk2", payload.unk2).Msg("RotateAgent")
	return nil
}

func (conn *GSConn) onMovementUpdate(payload *MovementUpdate) error {
	return nil
}

func (conn *GSConn) onLastPosBeforeMoveCancelled(payload *LastPosBeforeMoveCancelled) error {
	return nil
}

func (conn *GSConn) onUpdateTarget(payload *UpdateTarget) error {
	conn.log.Debug().Int("target", payload.targetAgentId).Str("playerName", conn.player.name).Msg("UpdateTarget")
	return nil
}

func (conn *GSConn) onInteractAgent(payload *InteractAgent) error {
	conn.player.SendChatWarning(fmt.Sprintf("missing interaction definition for agent=%d,action=%d", payload.agentId, payload.action))
	conn.log.Debug().Int("target", payload.agentId).Int("action", payload.action).Msg("InteractAgent")
	return nil
}

func (conn *GSConn) onCancelInteraction(payload *CancelInteraction) error {
	return nil
}

func (conn *GSConn) Close() {
	conn.closed = true
	if conn.player.connectedInstance != nil {
		(*conn.player.connectedInstance).RemovePlayer(&conn.player)
	}
	conn.socket.Close()
}

func (conn *GSConn) onClientPingRequest(payload *ClientPingRequest) error {
	return nil
}

func (conn *GSConn) onVerifyClientConnection(payload *VerifyClientConnection) error {
	conn.player.OnC2SVerifyConnection(*payload)
	return nil
}
func (conn *GSConn) onClientSeed(payload *ClientSeed) error {
	rc4Key, publicBytes := crypt.GenerateEncryptionKey([64]byte(payload.seed))

	var err error
	conn.dec, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return fmt.Errorf("error creating rc4 decrypter: %s", err)
	}
	resp := GwPacket.NewOutRaw()
	resp.Uint8(01)
	resp.Uint8(len(publicBytes) + 2)
	resp.Bytes(publicBytes[:])
	conn.WritePacket(&resp)

	conn.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return fmt.Errorf("error creating rc4 encrypter: %s", err)
	}

	(*conn.player.connectedInstance).AddPlayer(&conn.player)

	return nil
}

func (conn *GSConn) onGPUInformation(payload *GpuInformation) error {
	conn.log.Info().Str("name", payload.driverName).Str("version", payload.driverVersion).Msg("GPUInfo")
	return nil
}

func (conn *GSConn) onClientDisconnect(payload *ClientDisconnect) error {
	conn.player.OnUserDisconnected()
	conn.Close()
	return nil
}

func (conn *GSConn) onInstanceLoadRequestStart(payload *InstanceLoadRequestStart) error {
	conn.log.Debug().Msg("InstanceLoadRequestStart")
	return nil
}

func (conn *GSConn) onUpdateProfessionChoice(payload *UpdateProfessionChoice) error {
	conn.player.OnC2SUpdateProfessionChoice(*payload)
	// Now respond with updated items and profession
	return nil
}

func (conn *GSConn) onDyeEquipment(payload *DyeEquipment) error {
	conn.player.OnC2SDyeEquipment(*payload)
	return nil
}

func (conn *GSConn) onMapTravelToOutpost(payload *MapTravelToOutpost) error {
	conn.log.Info().Int("mapId", payload.mapId).Msg("MapTravel")
	return conn.player.connectedInstance.TransferPlayerToNewMap(&conn.player, payload.mapId)
}
