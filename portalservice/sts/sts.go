package sts

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

var logger = log.WithPrefix("sts")

type errorRespMsgPayload struct {
	XMLName xml.Name `xml:"Error"`
	Server  string   `xml:"server,attr"`
	Module  string   `xml:"module,attr"`
	Line    string   `xml:"line,attr"`
}

func (msg errorRespMsgPayload) Marshal() (string, error) {
	marshal, err := marshalXML(msg)
	if err != nil {
		return "", err
	}
	return strings.Replace(string(marshal), "></Error>", "/>", 1), nil
}

func NewErrorRespMsg(headerCode int, seqNumber int, server string, module string, line string) ([]byte, error) {
	header := RespHeader{Code: headerCode, Seq: seqNumber}
	payload := errorRespMsgPayload{
		Server: server,
		Module: module,
		Line:   line,
	}
	payloadStr, err := payload.Marshal()
	if err != nil {
		return nil, err
	}
	headerStr := header.Marshal(len(payloadStr) + trailingNewlineLen)
	return []byte(headerStr + payloadStr + "\n"), nil
}

type accountInfoMsgPayload struct {
	XMLName       xml.Name `xml:"Reply"`
	UserId        string
	UserCenter    int
	UserName      string
	ResumeToken   string
	EmailVerified int
}

func NewAccountInfoMsg(headerCode int, seqNumber int, userId string, userCenter int, userName string, resumeToken string, emailVerified int) ([]byte, error) {
	return MarshalResp(
		RespHeader{Code: headerCode, Seq: seqNumber},
		accountInfoMsgPayload{
			UserId:        userId,
			UserCenter:    userCenter,
			UserName:      userName,
			ResumeToken:   resumeToken,
			EmailVerified: emailVerified,
		},
	)
}

type row struct {
	XMLName  xml.Name `xml:"Row"`
	GameCode string
	Alias    string
	Created  string
}

type accountCreationInfoMsgPayload struct {
	XMLName xml.Name `xml:"Reply"`
	Type    string   `xml:"type,attr"`
	Rows    []row    `xml:"Row"`
}

func NewAccountCreationInfoMsg(headerCode int, seqNumber int, gameCode string, alias string, created string) ([]byte, error) {
	return MarshalResp(
		RespHeader{Code: headerCode, Seq: seqNumber},
		accountCreationInfoMsgPayload{
			Type: "array",
			Rows: []row{{
				GameCode: gameCode,
				Alias:    alias,
				Created:  created,
			}},
		},
	)
}

type gameTokenRespMsgPayload struct {
	XMLName xml.Name `xml:"Reply"`
	Token   string
}

func NewGameTokenMsg(headerCode int, seqNumber int, token string) ([]byte, error) {
	return MarshalResp(
		RespHeader{Code: headerCode, Seq: seqNumber},
		gameTokenRespMsgPayload{Token: token},
	)
}

// trailingNewlineLen accounts for the \n appended at the end of every message.
const trailingNewlineLen = 1

func MarshalResp(header RespHeader, payload any) ([]byte, error) {
	payloadStr, err := marshalXML(payload)
	if err != nil {
		return nil, err
	}
	headerStr := header.Marshal(len(payloadStr) + trailingNewlineLen)
	return []byte(headerStr + payloadStr + "\n"), nil
}

// indentSentinel is used as the indent parameter for xml.MarshalIndent.
// MarshalIndent inserts this string before each child element; we then strip
// it to produce XML with newlines but no indentation, matching the wire format.
const indentSentinel = "\x00"

func marshalXML(thing any) (string, error) {
	data, err := xml.MarshalIndent(thing, "", indentSentinel)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(data), indentSentinel, ""), nil
}

type PayloadConnect struct {
	ConnType    int
	Address     string
	ProductType int
	ProductName string
	AppIndex    int
	Epoch       int64
	Program     int
	Build       int
	Process     int
}

type PayloadLoginFinish struct {
	Language string
}

type PayloadListGameAccounts struct {
	GameCode string
}

type PayloadRequestGameToken struct {
	GameCode     string
	AccountAlias string
}

type RespHeader struct {
	Code int
	Seq  int
}

func (h RespHeader) codeString() string {
	switch h.Code {
	case 400:
		return "Success"
	case 200:
		return "OK"
	default:
		return "Unknown(" + strconv.Itoa(h.Code) + ")"
	}
}

func (h RespHeader) Marshal(contentLength int) string {
	var b strings.Builder
	b.WriteString("STS/1.0 ")
	b.WriteString(strconv.Itoa(h.Code))
	b.WriteByte(' ')
	b.WriteString(h.codeString())
	b.WriteString("\r\ns:")
	b.WriteString(strconv.Itoa(h.Seq))
	b.WriteString("R\r\nl:")
	b.WriteString(strconv.Itoa(contentLength))
	b.WriteString("\r\n\r\n")
	return b.String()
}

type ReqMsg struct {
	Header  ReqHeader
	Payload any
}

var stsInitialLineRE = regexp.MustCompile(`^([A-Za-z]) (/[^ ]+)`)
var stsLengthRE = regexp.MustCompile(`^l:([0-9]+)`)
var stsSeqRE = regexp.MustCompile(`^s:([0-9]+)`)

func (msg ReqMsg) Length() int {
	return msg.Header.HeaderLen + msg.Header.PayloadLen
}

type ReqHeader struct {
	Action     string
	Resource   string
	Seq        int
	PayloadLen int
	HeaderLen  int
}

const (
	pathConnect      = "/Sts/Connect"
	pathLoginFinish  = "/Auth/LoginFinish"
	pathListAccounts = "/Auth/ListMyGameAccounts"
	pathRequestToken = "/Auth/RequestGameToken"
)

var payloadRegistry = map[string]func() any{
	pathConnect:      func() any { return &PayloadConnect{} },
	pathLoginFinish:  func() any { return &PayloadLoginFinish{} },
	pathListAccounts: func() any { return &PayloadListGameAccounts{} },
	pathRequestToken: func() any { return &PayloadRequestGameToken{} },
}

func UnmarshalReqMsg(data []byte) (ReqMsg, error) {
	msg := ReqMsg{}

	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return msg, io.ErrUnexpectedEOF
	}
	headerBytes := data[:headerEnd]
	msg.Header.HeaderLen = headerEnd + 4

	lines := bytes.Split(headerBytes, []byte("\n"))
	if len(lines) == 0 {
		return msg, io.ErrUnexpectedEOF
	}

	initialLine := bytes.TrimRight(lines[0], "\r")
	matches := stsInitialLineRE.FindSubmatch(initialLine)
	if len(matches) != 3 {
		return msg, io.ErrUnexpectedEOF
	}
	msg.Header.Action = string(matches[1])
	msg.Header.Resource = string(matches[2])

	if err := unmarshalReqHeader(lines[1:], &msg.Header); err != nil {
		return msg, err
	}

	payloadStart := msg.Header.HeaderLen
	if payloadStart+msg.Header.PayloadLen > len(data) {
		return msg, io.ErrUnexpectedEOF
	}

	factory, ok := payloadRegistry[msg.Header.Resource]
	if !ok {
		return msg, nil
	}
	payload := factory()
	if err := xml.Unmarshal(data[payloadStart:payloadStart+msg.Header.PayloadLen], payload); err != nil {
		return msg, err
	}
	msg.Payload = payload
	return msg, nil
}

func unmarshalReqHeader(lines [][]byte, header *ReqHeader) error {
	foundLength := false
	foundSeq := false
	for _, ln := range lines {
		if !foundLength {
			if match := stsLengthRE.FindSubmatch(ln); len(match) == 2 {
				v, err := strconv.ParseInt(string(match[1]), 10, 32)
				if err != nil {
					return io.ErrUnexpectedEOF
				}
				header.PayloadLen = int(v)
				foundLength = true
			}
		}
		if !foundSeq {
			if match := stsSeqRE.FindSubmatch(ln); len(match) == 2 {
				v, err := strconv.ParseInt(string(match[1]), 10, 32)
				if err != nil {
					return io.ErrUnexpectedEOF
				}
				header.Seq = int(v)
				foundSeq = true
			}
		}
		if foundLength && foundSeq {
			break
		}
	}
	if !foundLength {
		return io.ErrUnexpectedEOF
	}
	return nil
}
