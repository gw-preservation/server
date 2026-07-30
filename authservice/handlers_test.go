package authservice

import (
	"gw1/server/db"
	"net"
	"os"
	"testing"

	GwPacket "gw1/server/gwpacket"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUUIDCounter byte = 0

func TestMain(m *testing.M) {
	if err := db.SetupTestDB(); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func setupTestConn(t *testing.T) (clientConn *net.TCPConn, conn *ASConn) {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	clientConn, err = net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	t.Cleanup(func() { clientConn.Close() })

	serverConn, err := listener.AcceptTCP()
	require.NoError(t, err)

	conn = NewASConn(serverConn, zerolog.Nop())
	return
}

func createTestAccount(t *testing.T, email string, charNames ...string) db.Account {
	t.Helper()
	testUUIDCounter++
	uuid := make([]byte, 16)
	uuid[15] = testUUIDCounter
	acc := db.Account{Email: email, Password: "p", PasswordSalt: []byte{0, 0, 0, 0, 0, 0, 0, 0}, UUID: uuid}
	require.NoError(t, db.CreateAccountForTest(&acc))
	for _, name := range charNames {
		bags := db.CreateDefaultBagsAndItems(0, 1, [7]int{})
		_, err := db.CreateCharacter(acc.ID, name, 1, 0, bags)
		require.NoError(t, err)
	}
	loaded, ok := db.GetFullAccountByID(acc.ID)
	require.True(t, ok)
	return loaded
}

func readResponse(t *testing.T, clientConn *net.TCPConn) (transactionId int, responseCode int) {
	t.Helper()
	buf := make([]byte, 256)
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 10) // opcode(2) + transactionId(4) + responseCode(4)

	in := GwPacket.NewIn(buf[:n])
	op, err := in.Uint16()
	require.NoError(t, err)
	assert.Equal(t, 0x0003, op, "expected RequestResponse opcode 0x0003")

	transactionId, err = in.Uint32()
	require.NoError(t, err)

	responseCode, err = in.Uint32()
	require.NoError(t, err)
	return
}

func flushOut(conn *ASConn) {
	if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}
}

func TestOnRenameCharacter_Success(t *testing.T) {
	clearTracker()
	acc := createTestAccount(t, "rename_ok@localhost", "OldName")
	conn, asConn := setupTestConn(t)
	_ = conn
	defer asConn.Close()

	asConn.acc = acc
	asConn.accountID = acc.ID

	payload := RenameCharacter{transactionId: 1, oldName: "OldName", newName: "NewName"}
	err := asConn.onRenameCharacter(&payload)
	require.NoError(t, err)
	flushOut(asConn)

	txnId, code := readResponse(t, conn)
	assert.Equal(t, 1, txnId)
	assert.Equal(t, 0, code)

	// Verify name changed in DB
	loaded, ok := db.GetFullAccountByID(acc.ID)
	require.True(t, ok)
	found := false
	for _, c := range loaded.Characters {
		if c.Name == "NewName" {
			found = true
		}
	}
	assert.True(t, found, "character should have new name")
}

func TestOnRenameCharacter_CharNotFound(t *testing.T) {
	clearTracker()
	acc := createTestAccount(t, "rename_notfound@localhost", "SomeChar")
	conn, asConn := setupTestConn(t)
	_ = conn
	defer asConn.Close()

	asConn.acc = acc
	asConn.accountID = acc.ID

	payload := RenameCharacter{transactionId: 2, oldName: "NonExistent", newName: "NewName"}
	err := asConn.onRenameCharacter(&payload)
	require.NoError(t, err)
	flushOut(asConn)

	txnId, code := readResponse(t, conn)
	assert.Equal(t, 2, txnId)
	assert.Equal(t, 49, code)
}

func TestOnRenameCharacter_NameTaken(t *testing.T) {
	clearTracker()
	acc := createTestAccount(t, "rename_taken@localhost", "MyChar", "OtherChar")
	conn, asConn := setupTestConn(t)
	_ = conn
	defer asConn.Close()

	asConn.acc = acc
	asConn.accountID = acc.ID

	payload := RenameCharacter{transactionId: 3, oldName: "MyChar", newName: "OtherChar"}
	err := asConn.onRenameCharacter(&payload)
	require.NoError(t, err)
	flushOut(asConn)

	txnId, code := readResponse(t, conn)
	assert.Equal(t, 3, txnId)
	assert.Equal(t, 29, code)
}

func TestOnRenameCharacter_WrongAccount(t *testing.T) {
	clearTracker()
	createTestAccount(t, "rename_wrong1@localhost", "TheirChar")
	acc2 := createTestAccount(t, "rename_wrong2@localhost")
	conn, asConn := setupTestConn(t)
	_ = conn
	defer asConn.Close()

	// acc2 tries to rename acc1's character
	asConn.acc = acc2
	asConn.accountID = acc2.ID

	payload := RenameCharacter{transactionId: 4, oldName: "TheirChar", newName: "StolenName"}
	err := asConn.onRenameCharacter(&payload)
	require.NoError(t, err)
	flushOut(asConn)

	txnId, code := readResponse(t, conn)
	assert.Equal(t, 4, txnId)
	assert.Equal(t, 49, code)
}

func TestOnRenameCharacter_UpdatesActiveCharacterName(t *testing.T) {
	clearTracker()
	acc := createTestAccount(t, "rename_active@localhost", "OldActive")
	conn, asConn := setupTestConn(t)
	_ = conn
	defer asConn.Close()

	asConn.acc = acc
	asConn.accountID = acc.ID
	asConn.activeCharacterName = "OldActive"

	payload := RenameCharacter{transactionId: 5, oldName: "OldActive", newName: "NewActive"}
	err := asConn.onRenameCharacter(&payload)
	require.NoError(t, err)
	flushOut(asConn)

	txnId, code := readResponse(t, conn)
	assert.Equal(t, 5, txnId)
	assert.Equal(t, 0, code)
	assert.Equal(t, "NewActive", asConn.activeCharacterName)
}
