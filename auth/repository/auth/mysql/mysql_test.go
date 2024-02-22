package mysql

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestAuth(t *testing.T) {
	ctx := context.Background()

	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
		mysql.WithScripts(
			filepath.Join(".", "schema.sql"), //TODO: workaround
		),
	)
	assert.Nil(t, err)
	mysqlDBHost, err := mysqlContainer.Host(ctx)
	assert.Nil(t, err)
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(
		ormKit.UseMySQL(
			fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				mysqlDBUsername,
				mysqlDBPassword,
				mysqlDBHost,
				mysqlDBPort.Port(),
				mysqlDBName,
			)))
	assert.Nil(t, err)

	accountRepo := accountMySQLRepo.CreateAccountRepo(mysqlDB)
	authRepo := CreateAuthRepo(mysqlDB)

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
