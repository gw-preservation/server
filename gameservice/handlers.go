package gameservice

import (
	"bytes"
	"crypto/rc4"
	"errors"
	"fmt"
	"gw1/server/crypt"
	"gw1/server/db"
	"gw1/server/geom"
	"gw1/server/packet"
	Item "gw1/server/item"
	"net"
)

type packetHandler func(*GSConn, *packet.In) (int, error)

func wrap[T any](
	unmarshal func(*packet.In) (T, error),
	handler func(*GSConn, *T) error,
) packetHandler {
	return func(conn *GSConn, in *packet.In) (int, error) {
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

var preGameHandlers = map[int]packetHandler{
	0x0500: wrap(UnmarshalVerifyClientConnection, (*GSConn).onVerifyClientConnection),
	0x4200: wrap(UnmarshalClientSeed, (*GSConn).onClientSeed),
	0x8008: wrap(UnmarshalClientDisconnect, (*GSConn).onClientDisconnect),
	0x8009: wrap(UnmarshalPingReply, (*GSConn).onPingReply),
	0x800a: wrap(UnmarshalGpuInformation, (*GSConn).onGPUInformation),
	0x800c: wrap(UnmarshalClientPingRequest, (*GSConn).onClientPingRequest),
	0x805f: wrap(UnmarshalUpdateProfessionChoice, (*GSConn).onUpdateProfessionChoice),
	0x8083: wrap(UnmarshalDyeEquipment, (*GSConn).onDyeEquipment),
	0x8088: wrap(UnmarshalCreateCharRequestPlayer, (*GSConn).onCreateCharRequestPlayer),
	0x8089: wrap(UnmarshalCreateCharRequestItems, (*GSConn).onCreateCharRequestItems),
	0x808a: wrap(UnmarshalCreateCharacterFinish, (*GSConn).onCreateCharacterFinish),
	0x8090: wrap(UnmarshalUnknown8090, (*GSConn).on8090),
	0x8091: wrap(UnmarshalUnknown8091, (*GSConn).on8091),
}

var inGameHandlers = map[int]packetHandler{
	0x8008: wrap(UnmarshalClientDisconnect, (*GSConn).onInInstanceDisconnect),
	0x8009: wrap(UnmarshalPingReply, (*GSConn).onPingReply),
	0x800a: wrap(UnmarshalGpuInformation, (*GSConn).onGPUInformation),
	0x800c: wrap(UnmarshalClientPingRequest, (*GSConn).onClientPingRequest),
	0x8027: wrap(UnmarshalCancelInteraction, (*GSConn).onCancelInteraction),
	0x802f: wrap(UnmarshalEquipItem, (*GSConn).onEquipItem),
	0x8038: wrap(UnmarshalInteractAgent, (*GSConn).onInteractAgent),
	0x803c: wrap(UnmarshalMovementUpdate, (*GSConn).onMovementUpdate),
	0x803d: wrap(UnmarshalMoveToPoint, (*GSConn).onMoveToPoint),
	0x803f: wrap(UnmarshalRotateAgent, (*GSConn).onRotateAgent),
	0x8046: wrap(UnmarshalLastPosBeforeMoveCancelled, (*GSConn).onLastPosBeforeMoveCancelled),
	0x8063: wrap(UnmarshalChatMessage, (*GSConn).onChatMessage),
	0x8068: wrap(UnmarshalDestroyItem, (*GSConn).onDestroyItem),
	0x8087: wrap(UnmarshalInstanceLoadRequestSpawnPoint, (*GSConn).onInstanceLoadRequestSpawnPoint),
	0x808f: wrap(UnmarshalInstanceLoadRequestPlayers, (*GSConn).onInstanceLoadRequestPlayers),
	0x80a0: wrap(UnmarshalPartyInvite, (*GSConn).onPartyInvite),
	0x80b0: wrap(UnmarshalMapTravelToOutpost, (*GSConn).onMapTravelToOutpost),
	0x80c0: wrap(UnmarshalUpdateTarget, (*GSConn).onUpdateTarget),
	0x8090: wrap(UnmarshalInstanceLoadRequestItems, (*GSConn).onInstanceLoadRequestItems),
	0x8091: wrap(UnmarshalUnknown8091, (*GSConn).on8091),
}

func (conn *GSConn) onInstanceLoadRequestItems(payload *InstanceLoadRequestItems) error {
	conn.log.Info().Msg("InstanceLoadRequestItems")
	conn.player.sendInstanceLoadItems()
	return nil
}

// In-instance handlers below run on the instance actor. They only record
// what the client asked for on the player; processPlayer applies in phase 2.

func (conn *GSConn) onInInstanceDisconnect(payload *ClientDisconnect) error {
	conn.player.pendingDisconnect = true
	return nil
}

func (conn *GSConn) onMoveToPoint(payload *MoveToPoint) error {
	conn.player.moveTo = payload
	return nil
}

func (conn *GSConn) onMovementUpdate(payload *MovementUpdate) error {
	if inst := conn.player.connectedInstance; inst != nil {
		inst.applyDirMovement(conn.player, geom.Pos2P{X: payload.posX, Y: payload.posY}, payload.facingX, payload.facingY, payload.dir)
	}
	return nil
}

func (conn *GSConn) onRotateAgent(payload *RotateAgent) error {
	return nil
}

func (conn *GSConn) onLastPosBeforeMoveCancelled(payload *LastPosBeforeMoveCancelled) error {
	if inst := conn.player.connectedInstance; inst != nil {
		inst.applyLastPosCorrection(conn.player, geom.Pos2P{X: payload.x, Y: payload.y, Plane: payload.unk2})
	}
	return nil
}

func (conn *GSConn) onChatMessage(payload *ChatMessage) error {
	conn.player.chat = payload
	return nil
}

func (conn *GSConn) onEquipItem(payload *EquipItem) error {
	conn.player.equip = payload
	return nil
}

func (conn *GSConn) onDestroyItem(payload *DestroyItem) error {
	conn.player.destroy = payload
	return nil
}

func (conn *GSConn) onInstanceLoadRequestSpawnPoint(payload *InstanceLoadRequestSpawnPoint) error {
	conn.player.loadSpawnRequested = true
	return nil
}

func (conn *GSConn) onInstanceLoadRequestPlayers(payload *InstanceLoadRequestPlayers) error {
	conn.player.loadPlayers = payload
	return nil
}

func (conn *GSConn) onMapTravelToOutpost(payload *MapTravelToOutpost) error {
	conn.player.mapTravel = payload
	return nil
}

func (conn *GSConn) onInteractAgent(payload *InteractAgent) error {
	conn.player.interact = payload
	return nil
}

func (conn *GSConn) onCancelInteraction(payload *CancelInteraction) error {
	conn.player.cancelInteractRequested = true
	return nil
}

func (conn *GSConn) onUpdateTarget(payload *UpdateTarget) error {
	conn.player.target = payload
	return nil
}

func (conn *GSConn) onPartyInvite(payload *PartyInvite) error {
	conn.log.Info().Str("name", payload.name).Msg("PartyInvite")
	return nil
}

// Pre-instance handlers (handshake + char creation, run on connection goroutine).

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
	resp := packet.NewOut(0xd)
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

func (conn *GSConn) onGPUInformation(payload *GpuInformation) error {
	conn.log.Info().Str("name", payload.driverName).Str("version", payload.driverVersion).Msg("GPUInfo")
	return nil
}

func (conn *GSConn) onClientDisconnect(payload *ClientDisconnect) error {
	conn.player.OnUserDisconnected()
	conn.Close()
	return nil
}

func (conn *GSConn) onCreateCharRequestItems(payload *CreateCharRequestItems) error {
	conn.log.Debug().Msg("CreateCharRequestItems")

	for _, itemid := range Item.DefaultEquipmentWarrior {
		itm, err := Item.GetItemDefinitionById(itemid)
		if err != nil {
			conn.log.Warn().Err(err).Msg("onCreateCharRequestItems: skipping unknown item")
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
	p.primaryProfession = payload.professionId
	numEquipSlots := p.itemMgr.GetNumSlotsInBag(1)
	for slotIndex := range numEquipSlots {
		p.itemMgr.RemoveItemInSlot(1, slotIndex)
	}
	p.EnqueuePacket(MarshalPlayerUpdateProfession(p.agentId, p.primaryProfession, 0))
	p.EnqueuePacket(MarshalSkillsUnlocked())

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

		p.connectedInstance = inst
	}
	p.isTransfer = info.IsTransfer
	if info.HasSpawnPoint {
		p.hasPendingSpawn = true
		p.pendingSpawn = geom.Pos2P{X: info.SpawnX, Y: info.SpawnY, Plane: info.SpawnPlane}
	}
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
	resp := packet.NewOutRaw()
	resp.Uint8(01)
	resp.Uint8(len(publicBytes) + 2)
	resp.Bytes(publicBytes[:])

	// The seed response must go out unencrypted, before conn.enc is enabled.
	// Enqueue it and flush synchronously through the writer goroutine.
	conn.EnqueuePacket(resp)
	if err := conn.Flush(); err != nil {
		return err
	}
	conn.enc, err = rc4.NewCipher(rc4Key[:])
	if err != nil {
		return fmt.Errorf("error creating rc4 encrypter: %s", err)
	}

	if conn.player.charCreationInProgress {
		conn.player.EnqueuePacket(MarshalInstanceLoadHead())
		conn.player.EnqueuePacket(MarshalCharCreationStart())
	} else if conn.player.connectedInstance != nil {
		conn.player.connectedInstance.AcceptPlayer(conn.player)
		conn.handedOver.Store(true)
	}

	conn.state = StateVerified

	return nil
}

func (conn *GSConn) onClientPingRequest(payload *ClientPingRequest) error {
	return nil
}
