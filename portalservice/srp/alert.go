package srp

import "fmt"

const (
	alertWarning uint8 = 1
	alertFatal   uint8 = 2
)

const (
	alertCloseNotify       uint8 = 0
	alertUnexpectedMessage uint8 = 10
	alertBadRecordMAC      uint8 = 20
	alertHandshakeFailure  uint8 = 40
	alertInternalError     uint8 = 80
)

type Alert struct {
	Level       uint8
	Description uint8
}

func ParseAlert(rec *Record) (*Alert, error) {
	if rec.Type != recordAlert {
		return nil, fmt.Errorf("not an Alert record")
	}

	if len(rec.Data) != 2 {
		return nil, fmt.Errorf("invalid Alert length")
	}

	return &Alert{
		Level:       rec.Data[0],
		Description: rec.Data[1],
	}, nil
}

func NewAlert(level, description uint8) *Record {
	return &Record{
		Type:    recordAlert,
		Version: tls12,
		Data: []byte{
			level,
			description,
		},
	}
}

func (a *Alert) Error() string {
	switch a.Description {
	case alertBadRecordMAC:
		return "TLS alert: bad_record_mac"
	case alertHandshakeFailure:
		return "TLS alert: handshake_failure"
	case alertUnexpectedMessage:
		return "TLS alert: unexpected_message"
	case alertCloseNotify:
		return "TLS alert: close_notify"
	default:
		return fmt.Sprintf(
			"TLS alert: level=%d description=%d",
			a.Level,
			a.Description,
		)
	}
}
