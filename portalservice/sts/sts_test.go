package sts

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAccountInfoMsg(t *testing.T) {
	expected := "STS/1.0 400 Success\r\ns:10R\r\nl:231\r\n\r\n<Reply>\n<UserId>12345678-1234-1234-1234-123456789012</UserId>\n<UserCenter>4</UserCenter>\n<UserName>:FakeUser.8019</UserName>\n<ResumeToken>12345678-1234-1234-1234-123456789013</ResumeToken>\n<EmailVerified>1</EmailVerified>\n</Reply>\n"

	result, err := NewAccountInfoMsg(400, 10, "12345678-1234-1234-1234-123456789012", 4, ":FakeUser.8019", "12345678-1234-1234-1234-123456789013", 1)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(result))
}

func TestNewAccountCreationInfoMsg(t *testing.T) {
	expected := "STS/1.0 200 OK\r\ns:3R\r\nl:127\r\n\r\n<Reply type=\"array\">\n<Row>\n<GameCode>gw1</GameCode>\n<Alias>gw1</Alias>\n<Created>2019-12-02T12:01:02Z</Created>\n</Row>\n</Reply>\n"

	result, err := NewAccountCreationInfoMsg(200, 3, "gw1", "gw1", "2019-12-02T12:01:02Z")
	assert.NoError(t, err)
	assert.Equal(t, expected, string(result))
}

func TestRespHeaderMarshal(t *testing.T) {
	tests := []struct {
		name          string
		code          int
		seq           int
		contentLength int
		expected      string
	}{
		{
			name:          "success with zero length",
			code:          400,
			seq:           1,
			contentLength: 0,
			expected:      "STS/1.0 400 Success\r\ns:1R\r\nl:0\r\n\r\n",
		},
		{
			name:          "OK with content",
			code:          200,
			seq:           5,
			contentLength: 123,
			expected:      "STS/1.0 200 OK\r\ns:5R\r\nl:123\r\n\r\n",
		},
		{
			name:          "unknown code",
			code:          503,
			seq:           0,
			contentLength: 10,
			expected:      "STS/1.0 503 Unknown(503)\r\ns:0R\r\nl:10\r\n\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := RespHeader{Code: tt.code, Seq: tt.seq}
			got := h.Marshal(tt.contentLength)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRespHeaderCodeString(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{400, "Success"},
		{200, "OK"},
		{500, "Unknown(500)"},
		{0, "Unknown(0)"},
	}
	for _, tt := range tests {
		h := RespHeader{Code: tt.code}
		got := h.codeString()
		assert.Equal(t, tt.expected, got, "codeString() for %d", tt.code)
	}
}

func TestNewErrorRespMsg(t *testing.T) {
	result, err := NewErrorRespMsg(400, 7, "1001", "2", "1146")
	assert.NoError(t, err)

	msg := string(result)
	assert.True(t, strings.HasPrefix(msg, "STS/1.0 400 Success\r\n"), "missing STS header prefix: %s", msg)
	assert.True(t, strings.Contains(msg, "s:7R\r\n"), "missing seq number: %s", msg)
	assert.True(t, strings.HasSuffix(msg, "\n"), "missing trailing newline: %s", msg)
	assert.True(t, strings.Contains(msg, "<Error"), "missing Error element: %s", msg)
	assert.True(t, strings.Contains(msg, "server=\"1001\""), "missing server attr: %s", msg)
	assert.True(t, strings.Contains(msg, "module=\"2\""), "missing module attr: %s", msg)
	assert.True(t, strings.Contains(msg, "line=\"1146\""), "missing line attr: %s", msg)
}

func TestNewErrorRespMsgSelfClosingTag(t *testing.T) {
	result, err := NewErrorRespMsg(400, 1, "s", "m", "l")
	assert.NoError(t, err)

	msg := string(result)
	assert.False(t, strings.Contains(msg, "></Error>"), "Error tag should be self-closing, got: %s", msg)
	assert.True(t, strings.Contains(msg, "/>"), "Error tag should end with />, got: %s", msg)
}

func TestNewGameTokenMsg(t *testing.T) {
	result, err := NewGameTokenMsg(200, 42, "my-secret-token")
	assert.NoError(t, err)

	msg := string(result)
	assert.True(t, strings.HasPrefix(msg, "STS/1.0 200 OK\r\n"), "missing STS header prefix: %s", msg)
	assert.True(t, strings.Contains(msg, "s:42R\r\n"), "missing seq number: %s", msg)
	assert.True(t, strings.Contains(msg, "<Token>my-secret-token</Token>"), "missing token element: %s", msg)
	assert.True(t, strings.Contains(msg, "<Reply>"), "missing Reply wrapper: %s", msg)
}

func TestMarshalResp(t *testing.T) {
	type simplePayload struct {
		XMLName xml.Name `xml:"Reply"`
		Value   string
	}
	header := RespHeader{Code: 200, Seq: 5}
	result, err := MarshalResp(header, simplePayload{Value: "test"})
	assert.NoError(t, err)

	msg := string(result)
	assert.True(t, strings.HasPrefix(msg, "STS/1.0 200 OK\r\n"), "missing header: %s", msg)
	assert.True(t, strings.Contains(msg, "<Value>test</Value>"), "missing payload content: %s", msg)
	assert.True(t, strings.HasSuffix(msg, "\n"), "missing trailing newline: %s", msg)
}

// buildReqMsg constructs a raw STS request message with correct length.
func buildReqMsg(action, resource string, seq int, payloadXML string) string {
	return fmt.Sprintf("%s %s\r\nl:%d\r\ns:%d\r\n\r\n%s", action, resource, len(payloadXML), seq, payloadXML)
}

func TestUnmarshalReqMsgConnect(t *testing.T) {
	payload := "<Connect><ConnType>1</ConnType><Address>localhost</Address></Connect>"
	raw := buildReqMsg("P", "/Sts/Connect", 1, payload)
	msg, err := UnmarshalReqMsg([]byte(raw))
	assert.NoError(t, err)
	assert.Equal(t, "P", msg.Header.Action)
	assert.Equal(t, "/Sts/Connect", msg.Header.Resource)
	assert.Equal(t, 1, msg.Header.Seq)
	assert.Equal(t, len(payload), msg.Header.PayloadLen)

	p, ok := msg.Payload.(*PayloadConnect)
	assert.True(t, ok, "Payload is %T, want *PayloadConnect", msg.Payload)
	assert.Equal(t, 1, p.ConnType)
	assert.Equal(t, "localhost", p.Address)
}

func TestUnmarshalReqMsgLoginFinish(t *testing.T) {
	payload := "<LoginFinish><Language>en</Language></LoginFinish>"
	raw := buildReqMsg("P", "/Auth/LoginFinish", 3, payload)
	msg, err := UnmarshalReqMsg([]byte(raw))
	assert.NoError(t, err)
	assert.Equal(t, "/Auth/LoginFinish", msg.Header.Resource)
	assert.Equal(t, 3, msg.Header.Seq)

	p, ok := msg.Payload.(*PayloadLoginFinish)
	assert.True(t, ok, "Payload is %T, want *PayloadLoginFinish", msg.Payload)
	assert.Equal(t, "en", p.Language)
}

func TestUnmarshalReqMsgListGameAccounts(t *testing.T) {
	payload := "<ListMyGameAccounts><GameCode>gw1</GameCode></ListMyGameAccounts>"
	raw := buildReqMsg("P", "/Auth/ListMyGameAccounts", 5, payload)
	msg, err := UnmarshalReqMsg([]byte(raw))
	assert.NoError(t, err)

	p, ok := msg.Payload.(*PayloadListGameAccounts)
	assert.True(t, ok, "Payload is %T, want *PayloadListGameAccounts", msg.Payload)
	assert.Equal(t, "gw1", p.GameCode)
}

func TestUnmarshalReqMsgRequestGameToken(t *testing.T) {
	payload := "<RequestGameToken><GameCode>gw1</GameCode><AccountAlias>test@example.com</AccountAlias></RequestGameToken>"
	raw := buildReqMsg("P", "/Auth/RequestGameToken", 7, payload)
	msg, err := UnmarshalReqMsg([]byte(raw))
	assert.NoError(t, err)
	assert.Equal(t, 7, msg.Header.Seq)

	p, ok := msg.Payload.(*PayloadRequestGameToken)
	assert.True(t, ok, "Payload is %T, want *PayloadRequestGameToken", msg.Payload)
	assert.Equal(t, "gw1", p.GameCode)
	assert.Equal(t, "test@example.com", p.AccountAlias)
}

func TestUnmarshalReqMsgEmptyData(t *testing.T) {
	_, err := UnmarshalReqMsg([]byte{})
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgBadInitialLine(t *testing.T) {
	_, err := UnmarshalReqMsg([]byte("bad data\r\nl:10\r\ns:1\r\n\r\n"))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgMissingHeaderEnd(t *testing.T) {
	_, err := UnmarshalReqMsg([]byte("P /Sts/Connect\r\nl:10\r\ns:1\r\n"))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgMissingLength(t *testing.T) {
	_, err := UnmarshalReqMsg([]byte("P /Sts/Connect\r\ns:1\r\n\r\n"))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgBadLength(t *testing.T) {
	_, err := UnmarshalReqMsg([]byte("P /Sts/Connect\r\nl:99999999999999999999\r\ns:1\r\n\r\n"))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgBadSeq(t *testing.T) {
	_, err := UnmarshalReqMsg([]byte("P /Sts/Connect\r\nl:10\r\ns:99999999999999999999\r\n\r\n"))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgTruncatedPayload(t *testing.T) {
	raw := "P /Sts/Connect\r\nl:200\r\ns:1\r\n\r\n<Connect>"
	_, err := UnmarshalReqMsg([]byte(raw))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestUnmarshalReqMsgUnknownResource(t *testing.T) {
	raw := buildReqMsg("P", "/Unknown/Path", 1, "<data></data>")
	msg, err := UnmarshalReqMsg([]byte(raw))
	assert.NoError(t, err)
	assert.Nil(t, msg.Payload)
}

func TestReqMsgLength(t *testing.T) {
	msg := ReqMsg{
		Header: ReqHeader{
			HeaderLen:  30,
			PayloadLen: 100,
		},
	}
	assert.Equal(t, 130, msg.Length())
}

func TestMustMarshalXMLSimple(t *testing.T) {
	type simple struct {
		XMLName xml.Name `xml:"Root"`
		Foo     string
	}
	result, err := marshalXML(simple{Foo: "bar"})
	assert.NoError(t, err)
	assert.Contains(t, result, "<Root>")
	assert.Contains(t, result, "<Foo>bar</Foo>")
}

func TestMustMarshalXMLWithAttributes(t *testing.T) {
	type withAttr struct {
		XMLName xml.Name `xml:"Error"`
		Server  string   `xml:"server,attr"`
	}
	result, err := marshalXML(withAttr{Server: "1001"})
	assert.NoError(t, err)
	assert.Contains(t, result, "server=\"1001\"")
}

func TestUnmarshalReqMsgSeqDefault(t *testing.T) {
	payload := "<LoginFinish><Language>en</Language></LoginFinish>"
	raw := fmt.Sprintf("P /Auth/LoginFinish\r\nl:%d\r\n\r\n%s", len(payload), payload)
	msg, err := UnmarshalReqMsg([]byte(raw))
	assert.NoError(t, err)
	assert.Equal(t, 0, msg.Header.Seq, "Seq should default to 0 when missing")
}

func TestUnmarshalReqMsgHeaderFieldsOrderIndependent(t *testing.T) {
	payload := "<LoginFinish><Language>en</Language></LoginFinish>"
	raw1 := fmt.Sprintf("P /Auth/LoginFinish\r\nl:%d\r\ns:3\r\n\r\n%s", len(payload), payload)
	raw2 := fmt.Sprintf("P /Auth/LoginFinish\r\ns:3\r\nl:%d\r\n\r\n%s", len(payload), payload)

	msg1, err := UnmarshalReqMsg([]byte(raw1))
	assert.NoError(t, err)
	msg2, err := UnmarshalReqMsg([]byte(raw2))
	assert.NoError(t, err)
	assert.Equal(t, msg1.Header.Seq, msg2.Header.Seq)
	assert.Equal(t, msg1.Header.PayloadLen, msg2.Header.PayloadLen)
}

func FuzzUnmarshalReqMsg(f *testing.F) {
	f.Add([]byte("P /Sts/Connect\r\nl:48\r\ns:1\r\n\r\n<Connect><ConnType>1</ConnType><Address>localhost</Address></Connect>"))
	f.Add([]byte("P /Auth/LoginFinish\r\nl:39\r\ns:3\r\n\r\n<LoginFinish><Language>en</Language></LoginFinish>"))
	f.Add([]byte("P /Auth/ListMyGameAccounts\r\nl:45\r\ns:5\r\n\r\n<ListMyGameAccounts><GameCode>gw1</GameCode></ListMyGameAccounts>"))
	f.Add([]byte("P /Auth/RequestGameToken\r\nl:69\r\ns:7\r\n\r\n<RequestGameToken><GameCode>gw1</GameCode><AccountAlias>test@example.com</AccountAlias></RequestGameToken>"))
	f.Add([]byte{})
	f.Add([]byte("\x00"))
	f.Add(bytes.Repeat([]byte("A"), 100000))

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := UnmarshalReqMsg(data)
		if err != nil {
			return
		}
		_ = msg.Length()
	})
}

func FuzzUnmarshalReqMsg_structured(f *testing.F) {
	resources := []string{
		"/Sts/Connect",
		"/Auth/LoginFinish",
		"/Auth/ListMyGameAccounts",
		"/Auth/RequestGameToken",
		"/Unknown/Path",
	}
	for _, r := range resources {
		f.Add([]byte("P"), []byte(r), []byte("<data></data>"))
		f.Add([]byte("P"), []byte(r), []byte(""))
	}

	f.Fuzz(func(t *testing.T, action []byte, resource []byte, payload []byte) {
		if len(action) != 1 || action[0] < 'A' || action[0] > 'Z' {
			return
		}
		raw := fmt.Sprintf("%s %s\r\nl:%d\r\ns:1\r\n\r\n%s", string(action), string(resource), len(payload), string(payload))
		msg, err := UnmarshalReqMsg([]byte(raw))
		if err != nil {
			return
		}
		_ = msg.Length()
	})
}

func FuzzRoundTrip(f *testing.F) {
	f.Add("P", "/Sts/Connect", 1, "<Connect><ConnType>1</ConnType><Address>localhost</Address></Connect>")
	f.Add("P", "/Auth/LoginFinish", 3, "<LoginFinish><Language>en</Language></LoginFinish>")
	f.Add("P", "/Auth/ListMyGameAccounts", 5, "<ListMyGameAccounts><GameCode>gw1</GameCode></ListMyGameAccounts>")
	f.Add("P", "/Auth/RequestGameToken", 7, "<RequestGameToken><GameCode>gw1</GameCode><AccountAlias>test@example.com</AccountAlias></RequestGameToken>")
	f.Add("P", "/Sts/Connect", 0, "<Connect></Connect>")
	f.Add("P", "/Unknown/Path", 1, "<data></data>")

	f.Fuzz(func(t *testing.T, action, resource string, seq int, payload string) {
		if len(action) != 1 || action[0] < 'A' || action[0] > 'Z' {
			return
		}
		if len(resource) < 2 || resource[0] != '/' || strings.ContainsAny(resource, " \t\n\r") {
			return
		}
		raw := buildReqMsg(action, resource, seq, payload)
		msg, err := UnmarshalReqMsg([]byte(raw))
		if err != nil {
			return
		}
		if msg.Header.Action != action {
			t.Errorf("Action = %q, want %q", msg.Header.Action, action)
		}
		if msg.Header.Resource != resource {
			t.Errorf("Resource = %q, want %q", msg.Header.Resource, resource)
		}
		if msg.Length() != len(raw) {
			t.Errorf("Length() = %d, want %d", msg.Length(), len(raw))
		}
	})
}

func FuzzMarshalResp(f *testing.F) {
	f.Add(200, 1, "gw1", "alias", "2024-01-01T00:00:00Z")
	f.Add(400, 0, "", "", "")
	f.Add(-1, 999999, "\x00\xff", "\xfe", "")

	f.Fuzz(func(t *testing.T, code, seq int, s1, s2, s3 string) {
		header := RespHeader{Code: code, Seq: seq}

		_, err := MarshalResp(header, accountInfoMsgPayload{
			UserId: s1, UserCenter: seq, UserName: s2, ResumeToken: s3, EmailVerified: seq,
		})
		if err != nil {
			return
		}

		_, err = MarshalResp(header, accountCreationInfoMsgPayload{
			Type: "array",
			Rows: []row{{GameCode: s1, Alias: s2, Created: s3}},
		})
		if err != nil {
			return
		}

		_, err = MarshalResp(header, gameTokenRespMsgPayload{Token: s1})
		if err != nil {
			return
		}

		_, err = MarshalResp(header, errorRespMsgPayload{
			Server: s1, Module: s2, Line: s3,
		})
		if err != nil {
			return
		}
	})
}
