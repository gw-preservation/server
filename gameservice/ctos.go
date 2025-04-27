package GameService

import (
	"fmt"
	GwPacket "gw1/server/gwpacket"
)

type createCharacterFinish struct {
	charName       string
	appearanceBits [8]byte
}

func unmarshalCreateCharacterFinish(in *GwPacket.In) (pl createCharacterFinish, err error) {
	pl.charName, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read char name: %w", err)
		return pl, err
	}
	varBytes, err := in.Bytes(8)
	if err != nil {
		err = fmt.Errorf("read appearance bits: %w", err)
		return pl, err
	}
	pl.appearanceBits = [8]byte(varBytes)
	return
}

type instanceLoadRequestSync struct {
	unkBytes [16]byte
}

func unmarshalInstanceLoadRequestSync(in *GwPacket.In) (pl instanceLoadRequestSync, err error) {
	varBytes, err := in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read unkBlob: %w", err)
		return
	}
	pl.unkBytes = [16]byte(varBytes)
	return
}

type _8091 struct {
	unk []byte
}

func unmarshal8091(in *GwPacket.In) (pl _8091, err error) {
	numBytes, err := in.Uint16()
	if err != nil {
		err = fmt.Errorf("read numBytes: %w", err)
		return
	}
	pl.unk, err = in.Bytes(numBytes)
	return
}

type verifyClientConnection struct {
	unk1          int // 0c 00
	clientVersion int // ae 92
	unk3          int // 00 00
	unk4          int // 01 00 00 00
	worldId       int
	mapId         int
	playerId      int
	accountUUID   [16]byte
	characterUUID [16]byte
	unk5          int
	unk6          int
}

func unmarshalVerifyClientConnection(in *GwPacket.In) (pl verifyClientConnection, err error) {
	pl.unk1, err = in.Uint16()
	if err != nil {
		err = fmt.Errorf("read unk1: %w", err)
		return
	}
	pl.clientVersion, err = in.Uint16()
	if err != nil {
		err = fmt.Errorf("read clientVersion: %w", err)
		return
	}
	pl.unk3, err = in.Uint16()
	if err != nil {
		err = fmt.Errorf("read unk3: %w", err)
		return
	}
	pl.unk4, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk4: %w", err)
		return
	}
	pl.worldId, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read worldId: %w", err)
		return
	}
	pl.mapId, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read mapId: %w", err)
		return
	}
	pl.playerId, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read playerId: %w", err)
		return
	}
	accountUUID, err := in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read accountUUID: %w", err)
		return
	}
	pl.accountUUID = [16]byte(accountUUID)

	characterUUID, err := in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read characterUUID: %w", err)
		return
	}
	pl.characterUUID = [16]byte(characterUUID)

	pl.unk5, err = in.Uint16()
	if err != nil {
		err = fmt.Errorf("read unk5: %w", err)
		return
	}
	pl.unk6, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk6: %w", err)
		return
	}
	return
}

type pingReply struct {
	unk int
}

func unmarshalPingReply(in *GwPacket.In) (pl pingReply, err error) {
	pl.unk, err = in.Uint32()
	return pl, err
}

/*
	CtoS packet(0x83) {
		Byte(2)  // Slot
		Byte(4)  // Dye color
	} endpacket(0x83)
*/
type dyeEquipment struct {
	slot     int
	dyeColor int
}

func unmarshalDyeEquipment(in *GwPacket.In) (pl dyeEquipment, err error) {
	pl.slot, err = in.Uint8()
	if err != nil {
		err = fmt.Errorf("read slot number: %w", err)
		return
	}
	pl.dyeColor, err = in.Uint8()
	if err != nil {
		err = fmt.Errorf("read dye color: %w", err)
		return
	}
	return
}

/*
	CtoS packet(0x5f) {
		Byte(1)  // PvE=0 PvP=1
		Byte(1)  // ProfessionID
	} endpacket(0x5f)
*/
type updateProfessionChoice struct {
	isPvE        bool
	professionId int
}

func unmarshalUpdateProfessionChoice(in *GwPacket.In) (pl updateProfessionChoice, err error) {
	isPvE, err := in.Uint8()
	if err != nil {
		err = fmt.Errorf("read isPvE: %w", err)
		return
	}
	pl.isPvE = isPvE > 0
	pl.professionId, err = in.Uint8()
	if err != nil {
		err = fmt.Errorf("read professionId: %w", err)
	}
	return
}

/*
	CtoS packet(0xa) {
		Blob(16) => 81 8b 34 e5 ae 72 fa 45 a9 50 71 f4 72 42 75 3e
		Dword(10)
		Dword(0)
		Dword(26100)
		Blob(12) => 41 75 74 68 65 6e 74 69 63 41 4d 44
		Dword(3841)
		Dword(12)
		Dword(7399)
		Dword(36896)
		Dword(4098)
		Dword(5686)
		Dword(3583)
		String(23) "AMD Radeon(TM) Graphics"
		String(15) "30.0.13040.9001G"
	} endpacket(0xa)
*/
type gpuInformation struct {
	unkBlob1      [16]byte
	unkDword1     int
	unkDword2     int
	unkDword3     int
	unkBlob2      [12]byte
	unkDword4     int
	unkDword5     int
	unkDword6     int
	unkDword7     int
	unkDword8     int
	unkDword9     int
	unkDword10    int
	driverName    string
	driverVersion string
}

func unmarshalGPUInformation(in *GwPacket.In) (pl gpuInformation, err error) {
	unkBlob1, err := in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read unkBlob1: %w", err)
		return
	}
	pl.unkBlob1 = [16]byte(unkBlob1)
	pl.unkDword1, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword1: %w", err)
		return
	}
	pl.unkDword2, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword2: %w", err)
		return
	}
	pl.unkDword3, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword3: %w", err)
		return
	}
	unkBlob2, err := in.Bytes(12)
	if err != nil {
		err = fmt.Errorf("read unkBlob1: %w", err)
		return
	}
	pl.unkBlob2 = [12]byte(unkBlob2)
	pl.unkDword4, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword4: %w", err)
		return
	}
	pl.unkDword5, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword5: %w", err)
		return
	}
	pl.unkDword6, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword6: %w", err)
		return
	}
	pl.unkDword7, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword7: %w", err)
		return
	}
	pl.unkDword8, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword8: %w", err)
		return
	}
	pl.unkDword9, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword9: %w", err)
		return
	}
	pl.unkDword10, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unkDword10: %w", err)
		return
	}
	pl.driverName, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read driverName: %w", err)
		return
	}
	pl.driverVersion, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read driverVersion: %w", err)
	}

	return
}

type clientSeed struct {
	seed [64]byte
}

func unmarshalClientSeed(in *GwPacket.In) (pl clientSeed, err error) {
	seed, err := in.Bytes(64)
	if err != nil {
		err = fmt.Errorf("read seed bytes: %w", err)
		return
	}
	pl.seed = [64]byte(seed)
	return
}

type chatMessage struct {
	agentId int
	message string
}

func unmarshalChatMessage(in *GwPacket.In) (pl chatMessage, err error) {
	pl.agentId, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read agentId: %w", err)
		return
	}
	pl.message, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read message: %w", err)
		return
	}
	return
}

type moveToPoint struct {
	x     float32
	y     float32
	plane int
}

func unmarshalMoveToPoint(in *GwPacket.In) (pl moveToPoint, err error) {
	pl.x, err = in.Float32()
	if err != nil {
		err = fmt.Errorf("read x: %w", err)
		return
	}
	pl.y, err = in.Float32()
	if err != nil {
		err = fmt.Errorf("read y: %w", err)
		return
	}
	pl.plane, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read plane: %w", err)
	}
	return
}
