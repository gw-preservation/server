package AuthService

import (
	"fmt"
	GwPacket "gw1/server/gwpacket"
)

type setActiveCharacter struct {
	reqNumber int
	charName  string
}

func unmarshalSetActiveCharacter(in *GwPacket.In) (pl setActiveCharacter, err error) {
	pl.reqNumber, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read reqNumber: %w", err)
		return
	}
	pl.charName, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read charName string: %w", err)
	}
	return
}

/*
// This one is to create new char

	CtoS packet(0x29) {
		Dword(3)
		Dword(11)
		Dword(0)
		Dword(0)
		Dword(2)
		Dword(0)
	} endpacket(0x29)

// This one is for login to char

	CtoS packet(0x29) {
	    Dword(3)
	    Dword(3)
	    Dword(148)
	    Dword(0)
	    Dword(2)
	    Dword(0)
	} endpacket(0x29)
*/
type loginCharacter struct {
	reqNumber int
	unk2      int
	mapId     int
	unk4      int
	unk5      int
	unk6      int
}

func unmarshalLoginCharacter(in *GwPacket.In) (pl loginCharacter, err error) {
	pl.reqNumber, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read reqNumber: %w", err)
		return
	}
	pl.unk2, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk2: %w", err)
		return
	}
	pl.mapId, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read mapId: %w", err)
		return
	}
	pl.unk4, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk4: %w", err)
		return
	}
	pl.unk5, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk5: %w", err)
		return
	}
	pl.unk6, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk6: %w", err)
	}
	return
}

type addAccessKey struct {
	reqNumber int
	accessKey string
}

func unmarshalAddAccessKey(in *GwPacket.In) (pl addAccessKey, err error) {
	pl.reqNumber, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read reqNumber: %w", err)
		return
	}
	pl.accessKey, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read accessKey: %w", err)
	}
	return
}

type clientHashInfo struct {
	clientVersion int
	unkHash       [16]byte
}

func unmarshalClientHashInfo(in *GwPacket.In) (pl clientHashInfo, err error) {
	pl.clientVersion, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read clientVersion: %w", err)
		return
	}
	unkHash, err := in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read unkHash: %w", err)
		return
	}
	pl.unkHash = [16]byte(unkHash)
	return
}

type clientVersionInfo struct {
	clientVersion int
	unk2          int
	unk3          int
}

func unmarshalClientVersionInfo(in *GwPacket.In) (pl clientVersionInfo, err error) {
	op, err := in.Uint16()
	if err != nil {
		err = fmt.Errorf("read op: %w", err)
		return
	}
	if op != 0x0400 {
		err = fmt.Errorf("bad opcode: %04x", op)
		return
	}
	remainingLen, err := in.Uint16()
	if err != nil {
		err = fmt.Errorf("read remainingLen: %w", err)
		return
	}
	if remainingLen != 12 {
		err = fmt.Errorf("bad remainingLen: %d", remainingLen)
		return
	}
	pl.clientVersion, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read clientVersion: %w", err)
		return
	}
	pl.unk2, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk2: %w", err)
		return
	}
	if pl.unk2 != 1 {
		err = fmt.Errorf("bad unk2: %08x", pl.unk2)
		return
	}
	pl.unk3, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk3: %w", err)
		return
	}
	if pl.unk3 != 4 {
		err = fmt.Errorf("bad unk3: %08x", pl.unk2)
		return
	}
	return
}

type _8000 struct {
	unk int
}

func unmarshal8000(in *GwPacket.In) (pl _8000, err error) {
	pl.unk, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk: %w", err)
	}
	return
}

type computerInfo struct {
	userName     string
	computerName string
}

func unmarshalComputerInfo(in *GwPacket.In) (pl computerInfo, err error) {
	pl.userName, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read userName: %w", err)
		return
	}
	pl.computerName, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read computerName: %w", err)
	}
	return
}

type _8023 struct {
	unk int
}

func unmarshal8023(in *GwPacket.In) (pl _8023, err error) {
	pl.unk, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read unk: %w", err)
	}
	return
}

/*
	CtoS packet(0x38) {
		Dword(1)
		Blob(16) => 14 b7 06 0e 24 b7 06 0e cc b4 06 0e df 48 8e 10
		Blob(16) => 68 95 7d 07 d0 a7 48 00 01 00 00 00 14 b7 06 0e
		String(9) ""
	} endpacket(0x38)
*/
type getAccountInfo struct {
	reqNumber                  int
	uuid1                      [16]byte
	gameTokenFromPortalService [16]byte
	unkString                  string
}

func unmarshalGetAccountInfo(in *GwPacket.In) (pl getAccountInfo, err error) {
	pl.reqNumber, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read reqNumber: %w", err)
		return
	}
	uuidBytes, err := in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read uuid1: %w", err)
		return
	}
	pl.uuid1 = [16]byte(uuidBytes)
	uuidBytes, err = in.Bytes(16)
	if err != nil {
		err = fmt.Errorf("read uuid2: %w", err)
		return
	}
	pl.gameTokenFromPortalService = [16]byte(uuidBytes)
	pl.unkString, err = in.UTF16WithLengthPrefix()
	if err != nil {
		err = fmt.Errorf("read unkString: %w", err)
	}
	return
}

type askServerResponse struct {
	reqNumber int
}

func unmarshalAskServerResponse(in *GwPacket.In) (pl askServerResponse, err error) {
	pl.reqNumber, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read reqNumber: %w", err)
	}
	return
}

type languageInfo struct {
	languageId int
}

func unmarshalLanguageInfo(in *GwPacket.In) (pl languageInfo, err error) {
	pl.languageId, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read languageId: %w", err)
	}
	return
}

type disconnect struct {
	errorCode int
}

func unmarshalDisconnect(in *GwPacket.In) (pl disconnect, err error) {
	pl.errorCode, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read errorCode: %w", err)
	}
	return
}

type setPlayerOnlineVisibilityStatus struct {
	newVisibility int
}

func unmarshalSetPlayerOnlineVisibilityStatus(in *GwPacket.In) (pl setPlayerOnlineVisibilityStatus, err error) {
	pl.newVisibility, err = in.Uint32()
	if err != nil {
		err = fmt.Errorf("read newVisibility: %w", err)
	}
	return
}

type clientSeed struct {
	seedBytes [64]byte
}

func unmarshalClientSeed(in *GwPacket.In) (pl clientSeed, err error) {
	_, err = in.Uint8()
	if err != nil {
		err = fmt.Errorf("read initial byte: %w", err)
		return
	}
	seedAndHeaderLen, err := in.Uint8()
	if err != nil {
		err = fmt.Errorf("read seed length: %w", err)
		return
	}
	if seedAndHeaderLen != 66 {
		err = fmt.Errorf("bad seedLen: %d", seedAndHeaderLen)
		return
	}
	seedBytes, err := in.Bytes(64)
	if err != nil {
		err = fmt.Errorf("read seed bytes: %w", err)
		return
	}
	pl.seedBytes = [64]byte(seedBytes)
	return
}
