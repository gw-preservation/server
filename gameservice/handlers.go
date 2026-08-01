package gameservice

import (
	"bytes"
	"crypto/rc4"
	"errors"
	"fmt"
	"gw1/server/crypt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	Item "gw1/server/item"
	"net"
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

// wrapP is like wrap but dispatches to a phase-1 intent setter on the Player.
// It is used by the in-instance handler table, which runs on the instance
// actor during the game tick and must not touch world state.
func wrapP[T any](
	unmarshal func(*GwPacket.In) (T, error),
	handler func(*Player, *T) error,
) packetHandler {
	return func(conn *GSConn, in *GwPacket.In) (int, error) {
		payload, err := unmarshal(in)
		if err != nil {
			return 0, err
		}
		if err := handler(conn.player, &payload); err != nil {
			return 0, err
		}
		return in.Position(), nil
	}
}

// handlers for the login handshake packets. Which of these is allowed at any
// given time is enforced by the connection state machine in conn.go. They run
// immediately on the connection goroutine via DrainHandshake.
var handshakeHandlers = map[int]packetHandler{
	0x0500: wrap(UnmarshalVerifyClientConnection, (*GSConn).onVerifyClientConnection),
	0x4200: wrap(UnmarshalClientSeed, (*GSConn).onClientSeed),
	0x8008: wrap(UnmarshalClientDisconnect, (*GSConn).onClientDisconnect),
}

// charCreationHandlers run immediately on the connection goroutine for a
// verified connection that has not yet joined an instance. They mutate only
// player-owned data (item manager, appearance), never instance state, so they
// are safe to process outside the instance actor.
var charCreationHandlers = map[int]packetHandler{
	0x8008: wrap(UnmarshalClientDisconnect, (*GSConn).onClientDisconnect),
	0x8009: wrap(UnmarshalPingReply, (*GSConn).onPingReply),
	0x800a: wrap(UnmarshalGpuInformation, (*GSConn).onGPUInformation),
	0x800c: wrap(UnmarshalClientPingRequest, (*GSConn).onClientPingRequest),
	0x805f: wrap(UnmarshalUpdateProfessionChoice, (*GSConn).onUpdateProfessionChoice),
	0x8083: wrap(UnmarshalDyeEquipment, (*GSConn).onDyeEquipment),
	0x8088: wrap(UnmarshalCreateCharRequestPlayer, (*GSConn).onCreateCharRequestPlayer),
	0x8089: wrap(UnmarshalCreateCharRequestArmors, (*GSConn).onCreateCharRequestArmors),
	0x808a: wrap(UnmarshalCreateCharacterFinish, (*GSConn).onCreateCharacterFinish),
	0x8090: wrap(UnmarshalUnknown8090, (*GSConn).on8090),
	0x8091: wrap(UnmarshalUnknown8091, (*GSConn).on8091),
}

// inInstanceHandlers run on the instance actor's game tick. Phase 1 setters
// only record intent on the player; the actor applies the intent in phase 2 of
// the same tick, so incoming packets never affect world state immediately.
// Stateless replies (ping, gpu) write responses directly during phase 1.
var inInstanceHandlers = map[int]packetHandler{
	0x8008: wrapP(UnmarshalClientDisconnect, (*Player).setDisconnectIntent),
	0x8009: wrap(UnmarshalPingReply, (*GSConn).onPingReply),
	0x800a: wrap(UnmarshalGpuInformation, (*GSConn).onGPUInformation),
	0x800c: wrap(UnmarshalClientPingRequest, (*GSConn).onClientPingRequest),
	0x8027: wrapP(UnmarshalCancelInteraction, (*Player).setCancelInteractionIntent),
	0x802f: wrapP(UnmarshalEquipItem, (*Player).setEquipIntent),
	0x8038: wrapP(UnmarshalInteractAgent, (*Player).setInteractIntent),
	0x803c: wrapP(UnmarshalMovementUpdate, (*Player).setMovementIntent),
	0x803d: wrapP(UnmarshalMoveToPoint, (*Player).setMoveIntent),
	0x803f: wrapP(UnmarshalRotateAgent, (*Player).setRotateIntent),
	0x8046: wrapP(UnmarshalLastPosBeforeMoveCancelled, (*Player).setLastPosCancelIntent),
	0x8063: wrapP(UnmarshalChatMessage, (*Player).setChatIntent),
	0x8068: wrapP(UnmarshalDestroyItem, (*Player).setDestroyIntent),
	0x8087: wrapP(UnmarshalInstanceLoadRequestSpawnPoint, (*Player).setLoadSpawnIntent),
	0x808f: wrapP(UnmarshalInstanceLoadRequestPlayers, (*Player).setLoadPlayersIntent),
	0x80a0: wrap(UnmarshalPartyInvite, (*GSConn).onPartyInvite),
	0x80b0: wrapP(UnmarshalMapTravelToOutpost, (*Player).setMapTravelIntent),
	0x80c0: wrapP(UnmarshalUpdateTarget, (*Player).setTargetIntent),
}

func (conn *GSConn) onPartyInvite(payload *PartyInvite) error {
	conn.log.Info().Str("name", payload.name).Msg("PartyInvite")
	return nil
}

func (conn *GSConn) onCreateCharRequestPlayer(payload *CreateCharRequestPlayer) error {
	conn.player.sendCreateCharacterInstanceInfo()
	return nil
}

func (conn *GSConn) on8090(payload *Unknown8090) error {
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

func (conn *GSConn) onCreateCharacterFinish(payload *CreateCharacterFinish) error {
	appearance := ParseAppearanceBits(uint32(payload.appearance))
	conn.log.Info().Str("desiredName", payload.name).Interface("appearance", appearance).Msg("CreateCharacterFinish")

	bags := db.CreateDefaultBagsAndItems(0, int(appearance.PrimaryProfession), conn.player.charCreationDyes)
	char, err := db.CreateCharacter(conn.player.dbAcc.ID, payload.name, int(appearance.PrimaryProfession), uint32(payload.appearance), bags)
	if errors.Is(err, db.ErrCharacterNameTaken) {
		conn.EnqueuePacket(MarshalCharCreationError(29))
		return nil
	}
	if err != nil {
		conn.log.Error().Err(err).Msg("failed to create character")
		return nil
	}

	conn.player.dbAcc.Characters = append(conn.player.dbAcc.Characters, char)

	varbs := []byte{}
	conn.EnqueuePacket(MarshalCharCreationFinish(char.UUID, payload.name, 148, varbs))

	return nil
}

func (conn *GSConn) onClientPingRequest(payload *ClientPingRequest) error {
	return nil
}

func (conn *GSConn) onVerifyClientConnection(payload *VerifyClientConnection) error {
	p := conn.player
	info, ok := ValidateConnectionToken(uint32(payload.securityTag))
	if !ok {
		p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("invalid securityTag")
		p.Disconnect()
		return nil
	}

	if info.InstanceTag != CharCreationTag && !bytes.Equal(payload.characterUUID[:], info.CharacterUUID[:]) {
		p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("character UUID does not match token")
		p.Disconnect()
		return nil
	}

	if !bytes.Equal(payload.accountUUID[:], info.AccountUUID[:]) {
		p.log.Error().Str("accUUID", db.UUIDStr(payload.accountUUID[:])).Msg("account UUID does not match token")
		p.Disconnect()
		return nil
	}

	expectedIP := net.ParseIP(info.ClientIP)
	actualIP := net.ParseIP(conn.clientIP())
	if expectedIP == nil || actualIP == nil || !expectedIP.Equal(actualIP) {
		p.log.Error().Str("expected", info.ClientIP).Str("got", conn.clientIP()).Msg("client IP does not match token")
		p.Disconnect()
		return nil
	}

	if info.InstanceTag == CharCreationTag {
		p.log.Debug().Msg("entering character creation")
		p.charCreationInProgress = true
		if payload.instanceTag != int(info.InstanceTag) {
			p.log.Error().Msg("char creation instanceTag mismatch")
			p.Disconnect()
			return nil
		}
	} else {
		inst, err := InstanceManager.GetOrCreateInstanceByMapId(payload.mapId)
		if inst == nil || err != nil {
			p.log.Error().Err(err).Msg("unable to create instance")
			p.Disconnect()
			return nil
		}

		if payload.instanceTag != int(info.InstanceTag) {
			p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("instanceTag does not match expected value")
			p.Disconnect()
			return nil
		}

		if info.InstanceTag != inst.GetTag() {
			p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("instance no longer has instanceTag specified by token")
			p.Disconnect()
			return nil
		}

		p.connectedInstance.Store(inst)
	}
	p.isTransfer = info.IsTransfer
	verified := false
	acc, ok := db.GetFullAccountByUUID(payload.accountUUID[:])
	if !ok {
		p.log.Error().Str("accUUID", db.UUIDStr(payload.accountUUID[:])).Msg("no such account")
		p.Disconnect()
		return nil
	}
	p.dbAcc = acc
	if !TrackAccount(acc.ID) {
		p.log.Error().Uint64("accountID", acc.ID).Msg("account already logged in")
		p.Disconnect()
		return nil
	}
	conn.accountID = acc.ID
	if p.charCreationInProgress {
		p.log.Debug().Msg("Skip UUID check - entering CharCreation")
	} else {
		// Check character UUID exists:
		for _, char := range acc.Characters {
			if bytes.Equal(char.UUID, payload.characterUUID[:]) {
				p.syncFromDB(char)
				verified = true
				break
			}
		}
		if !verified {
			p.log.Error().Str("characterUUID", db.UUIDStr(payload.characterUUID[:])).Msg("no such character")
			p.Disconnect()
			return nil
		}
	}

	// TODO: Here we should verify the map is adjacent to the LastOutpostID if its explorable!

	p.log.Debug().Str("instanceTag", fmt.Sprintf("%08x", payload.instanceTag)).Str("securityTag", fmt.Sprintf("%08x", payload.securityTag)).Int("mapId", payload.mapId).Int("unk3", payload.unk3).Int("unk4", payload.unk4).Int("unk5", payload.unk5).Int("unk6", payload.unk6).Msg("VerifyClientConnection")
	conn.state = StateAwaitClientSeed
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

	// The seed response must go out unencrypted, before conn.enc is enabled.
	// Hold the out mutex so the write can't interleave with a flush,
	// and so the cipher enable happens in the same critical section.
	conn.outMu.Lock()
	if err = conn.writeLocked(&resp); err != nil {
		conn.outMu.Unlock()
		return err
	}
	conn.enc, err = rc4.NewCipher(rc4Key[:])
	conn.outMu.Unlock()
	if err != nil {
		return fmt.Errorf("error creating rc4 encrypter: %s", err)
	}

	if conn.player.charCreationInProgress {
		conn.player.EnqueuePacket(MarshalInstanceLoadHead())
		conn.player.EnqueuePacket(MarshalCharCreationStart())
	} else if inst := conn.player.connectedInstance.Load(); inst != nil {
		if err := inst.AddPlayer(conn.player); err != nil {
			conn.log.Error().Err(err).Msg("failed to add player to instance")
		}
	}

	conn.state = StateVerified

	// Hand the input buffer over to the instance actor. The player is already
	// in the instance (AddPlayer above is synchronous), so the actor's next
	// game tick will drain buffered packets; character-creation connections
	// stay owned by the connection goroutine.
	if !conn.player.charCreationInProgress {
		conn.handedOver.Store(true)
	}

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

func (conn *GSConn) onCreateCharRequestArmors(payload *CreateCharRequestArmors) error {
	conn.log.Debug().Msg("CreateCharRequestArmors")

	for _, itemid := range Item.DefaultEquipmentWarrior {
		itm, err := Item.GetItemDefinitionById(itemid)
		if err != nil {
			conn.log.Warn().Err(err).Msg("onCreateCharRequestArmors: skipping unknown item")
			continue
		}
		itmlid, err := conn.player.itemMgr.AddItemToSlot(0, 0, itm, itemid, 0)
		if err != nil {
			conn.log.Error().Err(err).Msg("unable to add item to slot during char creation")
			return nil
		}
		slot := int(itm.GetEquipSlot())
		if err := conn.player.itemMgr.MoveItemByLocalId(itmlid, 1, slot); err != nil {
			conn.log.Error().Err(err).Msg("unable to move item to equipment slot during char creation")
			return nil
		}
	}
	return nil
}

func (conn *GSConn) onUpdateProfessionChoice(payload *UpdateProfessionChoice) error {
	p := conn.player
	if !p.charCreationInProgress {
		return nil
	}
	p.log.Debug().
		Int("profession", payload.professionId).
		Int("unk1", payload.unk1).
		Msg("UpdateProfessionChoice")
	if payload.professionId == p.primaryProfession {
		return nil
	}
	// TODO: validate profession id
	p.primaryProfession = payload.professionId
	// Marshal 1: delete equipped items
	// Remove items in equipped slots
	numEquipSlots := p.itemMgr.GetNumSlotsInBag(1)
	for slotIndex := range numEquipSlots {
		p.itemMgr.RemoveItemInSlot(1, slotIndex)
	}
	// Marshal 2: new appearance base?
	p.EnqueuePacket(MarshalPlayerUpdateProfession(p.agentId, p.primaryProfession, 0))
	p.EnqueuePacket(MarshalSkillsUnlocked())
	// Marshal 3: create new equipped items

	var equips []Item.ItemId
	switch payload.professionId {
	case 1:
		equips = Item.DefaultEquipmentWarrior
	case 2:
		equips = Item.DefaultEquipmentRanger
	case 3:
		equips = Item.DefaultEquipmentMonk
	case 4:
		equips = Item.DefaultEquipmentNecromancer
	case 5:
		equips = Item.DefaultEquipmentMesmer
	case 6:
		equips = Item.DefaultEquipmentElementalist
	}
	for _, itemid := range equips {
		itm, err := Item.GetItemDefinitionById(itemid)
		if err != nil {
			p.log.Warn().Err(err).Msg("onUpdateProfessionChoice: skipping unknown item")
			continue
		}
		itmlid, err := p.itemMgr.AddItemToSlot(0, 0, itm, itemid, 0)
		if err != nil {
			p.log.Error().Err(err).Msg("unable to add item to slot during profession change")
			return nil
		}
		err = p.itemMgr.MoveItemByLocalId(itmlid, 1, int(itm.GetEquipSlot()))
		if err != nil {
			p.log.Error().Err(err).Msg("unable to move item to equipment slot")
			return nil
		}
	}
	return nil
}

func (conn *GSConn) onDyeEquipment(payload *DyeEquipment) error {
	p := conn.player
	if !p.charCreationInProgress {
		return nil
	}
	if payload.color < 0 || payload.color > 9 {
		p.log.Warn().Int("color", payload.color).Msg("invalid dye color")
		return nil
	}
	if payload.slot < 0 || payload.slot > 6 {
		p.log.Warn().Int("slot", payload.slot).Msg("invalid dye slot")
		return nil
	}

	lid, err := p.itemMgr.GetLocalIdForSlot(equipmentBagIndex, payload.slot)
	if err != nil {
		p.log.Error().Err(err).Msg("error calling GetLocalIdForSlot")
		return nil
	}
	if lid == -1 {
		return nil
	}

	item, ok := p.itemMgr.GetItemByLocalId(lid)
	if !ok {
		p.log.Error().Int("lid", lid).Msg("item not found for local id")
		return nil
	}

	p.applyDyeToItem(lid, item, payload.color)
	p.charCreationDyes[payload.slot] = payload.color
	p.itemMgr.UpdateSlotDye(equipmentBagIndex, payload.slot, uint8(payload.color))
	return nil
}
