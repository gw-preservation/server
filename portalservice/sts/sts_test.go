package Sts

import (
	"testing"
)

func TestNewAccountInfoMsg(t *testing.T) {
	expected := "STS/1.0 400 Success\r\ns:10R\r\nl:231\r\n\r\n<Reply>\n<UserId>12345678-1234-1234-1234-123456789012</UserId>\n<UserCenter>4</UserCenter>\n<UserName>:FakeUser.8019</UserName>\n<ResumeToken>12345678-1234-1234-1234-123456789013</ResumeToken>\n<EmailVerified>1</EmailVerified>\n</Reply>\n"

	msg := string(NewAccountInfoMsg(400, 10, "12345678-1234-1234-1234-123456789012", 4, ":FakeUser.8019", "12345678-1234-1234-1234-123456789013", 1))
	if expected != msg {
		t.Fatalf("bad: %s\n", msg)
	}
}

func TestNewAccountCreationInfoMsg(t *testing.T) {
	expected := "STS/1.0 200 OK\r\ns:3R\r\nl:127\r\n\r\n<Reply type=\"array\">\n<Row>\n<GameCode>gw1</GameCode>\n<Alias>gw1</Alias>\n<Created>2019-12-02T12:01:02Z</Created>\n</Row>\n</Reply>\n"

	msg := string(NewAccountCreationInfoMsg(200, 3, "gw1", "gw1", "2019-12-02T12:01:02Z"))
	if expected != msg {
		t.Fatalf("bad: %s\n", expected)
	}
}
