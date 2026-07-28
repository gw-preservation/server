package srp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAlert_Valid(t *testing.T) {
	rec := &Record{
		Type: recordAlert,
		Data: []byte{alertWarning, alertBadRecordMAC},
	}

	alert, err := ParseAlert(rec)
	assert.NoError(t, err)
	assert.Equal(t, alertWarning, alert.Level)
	assert.Equal(t, alertBadRecordMAC, alert.Description)
}

func TestParseAlert_WrongType(t *testing.T) {
	rec := &Record{
		Type: recordHandshake,
		Data: []byte{1, 2},
	}

	_, err := ParseAlert(rec)
	assert.Error(t, err)
}

func TestParseAlert_InvalidLength(t *testing.T) {
	rec := &Record{
		Type: recordAlert,
		Data: []byte{1},
	}

	_, err := ParseAlert(rec)
	assert.Error(t, err)
}

func TestNewAlert(t *testing.T) {
	rec := NewAlert(alertFatal, alertHandshakeFailure)
	assert.Equal(t, recordAlert, rec.Type)
	assert.Equal(t, tls12, rec.Version)
	assert.Equal(t, []byte{alertFatal, alertHandshakeFailure}, rec.Data)
}

func TestAlert_Error_BadRecordMAC(t *testing.T) {
	a := &Alert{Level: alertFatal, Description: alertBadRecordMAC}
	assert.Equal(t, "TLS alert: bad_record_mac", a.Error())
}

func TestAlert_Error_HandshakeFailure(t *testing.T) {
	a := &Alert{Level: alertFatal, Description: alertHandshakeFailure}
	assert.Equal(t, "TLS alert: handshake_failure", a.Error())
}

func TestAlert_Error_UnexpectedMessage(t *testing.T) {
	a := &Alert{Level: alertFatal, Description: alertUnexpectedMessage}
	assert.Equal(t, "TLS alert: unexpected_message", a.Error())
}

func TestAlert_Error_CloseNotify(t *testing.T) {
	a := &Alert{Level: alertWarning, Description: alertCloseNotify}
	assert.Equal(t, "TLS alert: close_notify", a.Error())
}

func TestAlert_Error_Unknown(t *testing.T) {
	a := &Alert{Level: 1, Description: 99}
	assert.Contains(t, a.Error(), "level=1")
	assert.Contains(t, a.Error(), "description=99")
}
