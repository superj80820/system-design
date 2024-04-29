package mysql

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
)

func TestAuth(t *testing.T) {
	ctx := context.Background()

	mysqlContainer, err := mysqlContainer.CreateMySQL(ctx, mysqlContainer.UseSQLSchema(
		filepath.Join("../../account/mysql", "schema.sql"),
		filepath.Join(".", "schema.sql"),
	))
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(ormKit.UseMySQL(mysqlContainer.GetURI()))
	assert.Nil(t, err)

	accountRepo := accountMySQLRepo.CreateAccountRepo(mysqlDB)
	authRepo, err := CreateAuthRepo(mysqlDB,
		`-----BEGIN PRIVATE KEY-----\nMHcCAQEEIMWqJOv9daWpi0afZJS4s1uNOVpY71xrANwVYJyuQv94oAoGCCqGSM49\nAwEHoUQDQgAEUcdKl8oCd6/YD8EJD35vNUVLn4RvcOf6v5+aMzsH58Y1BJ51YZyw\nbEjwc8g0ygLGcliyIIDXYoWXYumXxRHbbA==\n-----END PRIVATE KEY-----`,
		`-----BEGIN PRIVATE KEY-----\nMHcCAQEEIBLuZnd1jEf20jkDRmSn7nPsI2Z89SySitRG0qUefJQNoAoGCCqGSM49\nAwEHoUQDQgAEnPYFR9O0EPRHfvgoOVoaw3BzBMGEZLSz+hYDTkrzf4DTrvDDT7JK\nxfeM9buQA+fbKAfR5rFBQmOTNuPNWZFY1w==\n-----END PRIVATE KEY-----`,
	)
	assert.Nil(t, err)

	email := "email@gmail.com"
	password := "password"
	token := "token"
	expireAt := time.Now().Add(10 * time.Minute)

	account, err := accountRepo.Create(email, password)
	assert.Nil(t, err)

	tokenInstance, err := authRepo.CreateToken(account.ID, token, expireAt, domain.REFRESH_TOKEN)
	assert.Nil(t, err)
	assert.Equal(t, account.ID, tokenInstance.UserID)
	assert.Equal(t, token, tokenInstance.Token)
	assert.Equal(t, domain.ACTIVE, tokenInstance.Status)
	// assert.Equal(t, expireAt.Format(time.DateTime), tokenInstance.ExpireAt.Format(time.DateTime)) // TODO: figure out why miss by a second

	tokenInstance, err = authRepo.GetLastRefreshTokenByUserID(account.ID)
	assert.Nil(t, err)
	assert.Equal(t, account.ID, tokenInstance.UserID)
	assert.Equal(t, token, tokenInstance.Token)
	assert.Equal(t, domain.ACTIVE, tokenInstance.Status)
	// assert.Equal(t, expireAt.Format(time.DateTime), tokenInstance.ExpireAt.Format(time.DateTime)) // TODO: figure out why miss by a second

	err = authRepo.UpdateStatusToken(tokenInstance.Token, domain.REVOKE)
	assert.Nil(t, err)

	tokenInstance, err = authRepo.GetLastRefreshTokenByUserID(account.ID)
	assert.Nil(t, err)
	assert.Equal(t, account.ID, tokenInstance.UserID)
	assert.Equal(t, token, tokenInstance.Token)
	assert.Equal(t, domain.REVOKE, tokenInstance.Status)
	// assert.Equal(t, expireAt.Format(time.DateTime), tokenInstance.ExpireAt.Format(time.DateTime)) // TODO: figure out why miss by a second
}
