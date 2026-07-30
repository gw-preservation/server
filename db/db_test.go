package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := SetupTestDB(); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestAddDbChar_Success(t *testing.T) {
	acc := Account{Email: "test@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := AddDbChar(acc.ID, "TestChar", 1, 0, bags)
	require.NoError(t, err)
	assert.NotZero(t, char.ID)
	assert.Equal(t, "TestChar", char.Name)
	assert.Equal(t, acc.ID, char.AccountID)

	var loaded Character
	require.NoError(t, database.Preload("Bags.Slots").First(&loaded, char.ID).Error)
	assert.Equal(t, "TestChar", loaded.Name)
	assert.Len(t, loaded.Bags, 2)
}

func TestAddDbChar_ErrorReturned(t *testing.T) {
	acc := Account{Email: "adderr@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := AddDbChar(acc.ID, "AddErrChar", 1, 0, bags)
	require.NoError(t, err)
	assert.NotZero(t, char.ID)
}

func TestCreateCharacter_Success(t *testing.T) {
	acc := Account{Email: "create_success@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := CreateCharacter(acc.ID, "CreateSuccess", 1, 0, bags)
	require.NoError(t, err)
	assert.NotZero(t, char.ID)
	assert.Equal(t, "CreateSuccess", char.Name)

	var loaded Character
	require.NoError(t, database.Preload("Bags.Slots").First(&loaded, char.ID).Error)
	assert.Equal(t, "CreateSuccess", loaded.Name)
	assert.Len(t, loaded.Bags, 2)
}

func TestCreateCharacter_DuplicateName(t *testing.T) {
	acc := Account{Email: "create_dup@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc.ID, "CreateDup", 1, 0, bags)
	require.NoError(t, err)

	_, err = CreateCharacter(acc.ID, "CreateDup", 1, 0, bags)
	assert.ErrorIs(t, err, ErrCharacterNameTaken)
}

func TestCreateCharacter_DifferentAccountSameName(t *testing.T) {
	acc1 := Account{Email: "dsn1@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc1).Error)
	acc2 := Account{Email: "dsn2@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc2).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc1.ID, "GlobalUniqueName", 1, 0, bags)
	require.NoError(t, err)

	_, err = CreateCharacter(acc2.ID, "GlobalUniqueName", 1, 0, bags)
	assert.ErrorIs(t, err, ErrCharacterNameTaken)
}

func TestSaveCharacterMapTransfer_CreatesBagsAndUpdatesOutpost(t *testing.T) {
	acc := Account{Email: "transfer_test@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := CreateCharacter(acc.ID, "TransferChar", 1, 0, bags)
	require.NoError(t, err)

	newBags := []Bag{
		{
			Capacity: 5,
			Type:     1,
			Slots: []Slot{
				{BagID: 0, ItemID: 100, ItemQuantity: 1},
			},
		},
	}
	err = SaveCharacterMapTransfer(char.ID, 999, newBags)
	require.NoError(t, err)

	var loaded Character
	require.NoError(t, database.Preload("Bags.Slots").First(&loaded, char.ID).Error)
	assert.Equal(t, uint16(999), loaded.LastOutpostID)
	require.Len(t, loaded.Bags, 1)
	require.Len(t, loaded.Bags[0].Slots, 1)
	assert.Equal(t, uint32(100), loaded.Bags[0].Slots[0].ItemID)
}

func TestSaveCharacterMapTransfer_RollbackOnError(t *testing.T) {
	acc := Account{Email: "rollback_test@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := CreateCharacter(acc.ID, "RollbackChar", 1, 0, bags)
	require.NoError(t, err)

	originalOutpost := char.LastOutpostID

	newBags := []Bag{
		{
			Capacity: 5,
			Type:     1,
			Slots: []Slot{
				{BagID: 0, ItemID: 100, ItemQuantity: 1},
			},
		},
	}
	// Use a non-existent character ID to trigger rollback
	err = SaveCharacterMapTransfer(9999999, 999, newBags)
	require.Error(t, err)

	var loaded Character
	require.NoError(t, database.First(&loaded, char.ID).Error)
	assert.Equal(t, originalOutpost, loaded.LastOutpostID)
}

func TestDeleteDbChar_Success(t *testing.T) {
	acc := Account{Email: "delete_success@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := CreateCharacter(acc.ID, "DeleteMe", 1, 0, bags)
	require.NoError(t, err)

	err = DeleteDbChar("DeleteMe", acc.ID)
	assert.NoError(t, err)

	var count int64
	database.Model(&Character{}).Where("id = ?", char.ID).Count(&count)
	assert.Zero(t, count)
}

func TestDeleteDbChar_WrongAccount(t *testing.T) {
	acc := Account{Email: "delete_wrong@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)
	otherAcc := Account{Email: "delete_wrong2@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&otherAcc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc.ID, "NotYours", 1, 0, bags)
	require.NoError(t, err)

	err = DeleteDbChar("NotYours", otherAcc.ID)
	assert.Error(t, err)
}

func TestSetLastOutpostForChar_Success(t *testing.T) {
	acc := Account{Email: "outpost@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	char, err := CreateCharacter(acc.ID, "OutpostChar", 1, 0, bags)
	require.NoError(t, err)
	assert.NotEqual(t, uint16(444), char.LastOutpostID)

	err = SetLastOutpostForChar(char.ID, 444)
	assert.NoError(t, err)

	var loaded Character
	database.First(&loaded, char.ID)
	assert.Equal(t, uint16(444), loaded.LastOutpostID)
}

func TestSetLastOutpostForChar_NotFound(t *testing.T) {
	err := SetLastOutpostForChar(9999999, 444)
	assert.Error(t, err)
}

func TestCharacterNameExists(t *testing.T) {
	acc := Account{Email: "exists_test@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	assert.False(t, CharacterNameExists("NonExistentChar"))

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc.ID, "ExistsChar", 1, 0, bags)
	require.NoError(t, err)

	assert.True(t, CharacterNameExists("ExistsChar"))
}

func TestRenameCharacter_Success(t *testing.T) {
	acc := Account{Email: "rename_success@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc.ID, "OldNameRS", 1, 0, bags)
	require.NoError(t, err)

	err = RenameCharacter("OldNameRS", "NewNameRS", acc.ID)
	assert.NoError(t, err)

	var loaded Character
	require.NoError(t, database.Where("account_id = ?", acc.ID).First(&loaded).Error)
	assert.Equal(t, "NewNameRS", loaded.Name)
}

func TestRenameCharacter_CharNotFound(t *testing.T) {
	acc := Account{Email: "rename_notfound@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	err := RenameCharacter("NonExistent", "NewNameNF", acc.ID)
	assert.ErrorIs(t, err, ErrCharacterNotFound)
}

func TestRenameCharacter_WrongAccount(t *testing.T) {
	acc1 := Account{Email: "rename_wrong1@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc1).Error)
	acc2 := Account{Email: "rename_wrong2@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc2).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc1.ID, "NotYoursWA", 1, 0, bags)
	require.NoError(t, err)

	err = RenameCharacter("NotYoursWA", "NewNameWR", acc2.ID)
	assert.ErrorIs(t, err, ErrCharacterNotFound)
}

func TestRenameCharacter_NewNameTaken(t *testing.T) {
	acc := Account{Email: "rename_taken@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc.ID, "OldNameNT", 1, 0, bags)
	require.NoError(t, err)
	_, err = CreateCharacter(acc.ID, "TakenName", 1, 0, bags)
	require.NoError(t, err)

	err = RenameCharacter("OldNameNT", "TakenName", acc.ID)
	assert.ErrorIs(t, err, ErrCharacterNameTaken)
}

func TestRenameCharacter_SameName(t *testing.T) {
	acc := Account{Email: "rename_same@localhost", Password: "p", PasswordSalt: randSalt(), UUID: randUuid()}
	require.NoError(t, database.Create(&acc).Error)

	bags := CreateDefaultBagsAndItems(0, 1, [7]int{})
	_, err := CreateCharacter(acc.ID, "SameName", 1, 0, bags)
	require.NoError(t, err)

	err = RenameCharacter("SameName", "SameName", acc.ID)
	assert.NoError(t, err)

	var loaded Character
	require.NoError(t, database.Where("account_id = ?", acc.ID).First(&loaded).Error)
	assert.Equal(t, "SameName", loaded.Name)
}
