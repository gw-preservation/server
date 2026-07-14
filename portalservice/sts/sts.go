package Sts

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

var logger = log.WithPrefix("Sts")

type errorRespMsgPayload struct {
	XMLName xml.Name `xml:"Error"`
	Server  string   `xml:"server,attr"`
	Module  string   `xml:"module,attr"`
	Line    string   `xml:"line,attr"`
}

func (msg errorRespMsgPayload) Marshal() string {
	// Custom logic due to needing a self-closing tag
	marshal := mustMarshalXML(msg)
	return strings.Replace(string(marshal), "></Error>", "/>", 1)
}

func newRespHeader(code, seq int) RespHeader {
	return RespHeader{
		Code: code,
		Seq:  seq,
	}
}

func NewErrorRespMsg(headerCode int, seqNumber int, server string, module string, line string) []byte {
	header := newRespHeader(headerCode, seqNumber)
	payload := errorRespMsgPayload{
		Server: server,
		Module: module,
		Line:   line,
	}
	// Custom logic due to needing a self-closing tag
	headerStr := header.Marshal()
	payloadStr := payload.Marshal()
	headerStr = fmt.Sprintf(headerStr, len(payloadStr)+1) //+1 due to \n at end of message
	return []byte(headerStr + payloadStr + "\n")
}

// Wrapped with <Reply></Reply>
type accountInfoMsgPayload struct {
	XMLName       xml.Name `xml:"Reply"`
	UserId        string
	UserCenter    int
	UserName      string
	ResumeToken   string
	EmailVerified int
}

func NewAccountInfoMsg(headerCode int, seqNumber int, userId string, userCenter int, userName string, resumeToken string, emailVerified int) []byte {
	header := newRespHeader(headerCode, seqNumber)
	payload := accountInfoMsgPayload{
		UserId:        userId,
		UserCenter:    userCenter,
		UserName:      userName,
		ResumeToken:   resumeToken,
		EmailVerified: emailVerified,
	}
	return MarshalResp(header, payload)
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

func NewAccountCreationInfoMsg(headerCode int, seqNumber int, gameCode string, alias string, created string) []byte {
	header := newRespHeader(headerCode, seqNumber)
	payload := accountCreationInfoMsgPayload{
		Type: "array",
		Rows: []row{{
			GameCode: gameCode,
			Alias:    alias,
			Created:  created,
		}},
	}
	return MarshalResp(header, payload)
}

type gameTokenRespMsgPayload struct {
	XMLName xml.Name `xml:"Reply"`
	Token   string
}

func NewGameTokenMsg(headerCode int, seqNumber int, token string) []byte {
	header := newRespHeader(headerCode, seqNumber)
	payload := gameTokenRespMsgPayload{
		Token: token,
	}
	return MarshalResp(header, payload)
}
func MarshalResp(header RespHeader, payload any) []byte {
	payloadStr := mustMarshalXML(payload)
	headerStr := fmt.Sprintf(header.Marshal(), len(payloadStr)+1)
	return []byte(headerStr + payloadStr + "\n")
}

func mustMarshalXML(thing any) string {
	data, err := xml.MarshalIndent(thing, "", "\xCA\xFE\xBA\xBE")
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(data), "\xCA\xFE\xBA\xBE", "")
}

func unmarshalPayload(data []byte, v any) error {
	return xml.Unmarshal(data, v)
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
		return "Unknown"
	}
}

func (h RespHeader) Marshal() string {
	return fmt.Sprintf(
		"STS/1.0 %d %s\r\ns:%dR\r\nl:%%d\r\n\r\n",
		h.Code,
		h.codeString(),
		h.Seq,
	)
}

type ReqMsg struct {
	Header  ReqHeader
	Payload any
}

var stsInitialLineRE = regexp.MustCompile(`^([A-Za-z]) (/[^ ]+)`)
var stsLengthRE = regexp.MustCompile("^l:([0-9]+)")
var stsSeqRE = regexp.MustCompile("^s:([0-9]+)")

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

func UnmarshalReqMsg(data []byte) (ReqMsg, error) {
	msg := ReqMsg{}
	str := string(data)
	lines := strings.Split(str, "\n")
	if len(lines) == 0 {
		return msg, io.ErrUnexpectedEOF
	}
	initialLine := lines[0]
	// Parse initial line (Action+Resource)
	matches := stsInitialLineRE.FindStringSubmatch(initialLine)
	if len(matches) != 3 {
		return msg, io.ErrUnexpectedEOF
	}
	msg.Header.Action = matches[1]
	msg.Header.Resource = matches[2]
	// Find the End-Of-Header separator line
	headerEndLineNumber := -1
	for i, line := range lines {
		if line == "\r" {
			headerEndLineNumber = i
			break
		}
	}
	if headerEndLineNumber == -1 {
		logger.Debug("lacking End-Of-Header line in Sts message")
		return msg, io.ErrUnexpectedEOF
	}
	err := unmarshalReqHeader(lines[:headerEndLineNumber], &msg.Header)
	if err != nil {
		return msg, err
	}
	payloadStartIndex := strings.Index(str, "\r\n\r\n") + 4
	msg.Header.HeaderLen = payloadStartIndex
	remainingBytes := len(data) - payloadStartIndex
	//logger.Infof("Remaining data: %s", str[payloadStartIndex:])
	if payloadStartIndex+remainingBytes < len(data) {
		return msg, io.ErrUnexpectedEOF // Need more data to fit payload
	}
	switch msg.Header.Resource {
	case pathConnect:
		payload := &PayloadConnect{}
		err = unmarshalPayload(data[payloadStartIndex:], payload)
		msg.Payload = payload
	case pathLoginFinish:
		payload := &PayloadLoginFinish{}
		err = unmarshalPayload(data[payloadStartIndex:], payload)
		msg.Payload = payload
	case pathListAccounts:
		payload := &PayloadListGameAccounts{}
		err = unmarshalPayload(data[payloadStartIndex:], payload)
		msg.Payload = payload
	case pathRequestToken:
		payload := &PayloadRequestGameToken{}
		err = unmarshalPayload(data[payloadStartIndex:], payload)
		msg.Payload = payload
	}
	if err != nil {
		return msg, err
	}
	return msg, nil
}

func unmarshalReqHeader(lines []string, header *ReqHeader) error {
	foundLengthLine := false
	foundSeqLine := false
	for _, ln := range lines {
		if !foundLengthLine {
			match := stsLengthRE.FindStringSubmatch(ln)
			if len(match) == 2 {
				foundLengthLine = true
				lenStr := match[1]
				lenInt, err := strconv.ParseInt(lenStr, 10, 32)
				if err != nil {
					return fmt.Errorf("bad length number: %s", lenStr)
				}
				header.PayloadLen = int(lenInt)
			}
		}
		if !foundSeqLine {
			match := stsSeqRE.FindStringSubmatch(ln)
			if len(match) == 2 {
				foundSeqLine = true
				seqStr := match[1]
				seqInt, err := strconv.ParseInt(seqStr, 10, 32)
				if err != nil {
					return fmt.Errorf("bad seq number: %s", seqStr)
				}
				header.Seq = int(seqInt)
			}
		}
	}
	if !foundLengthLine {
		// Mandatory
		return errors.New("missing length line in Sts header")
	}
	return nil
}
