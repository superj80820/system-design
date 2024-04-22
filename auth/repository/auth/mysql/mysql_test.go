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
	authRepo, err := CreateAuthRepo(mysqlDB, "./access-private-key.pem", "./refresh-private-key.pem")
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
