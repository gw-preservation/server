package srp

import "errors"

const (
	TLS_SRP_SHA_WITH_AES_256_CBC_SHA uint16 = 0xc020
)

const (
	compressionNull uint8 = 0
)

const (
	extensionSRP uint16 = 12
)

var (
	ErrBadRecordMAC = errors.New("bad record MAC")
)
