package usecase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	authMySQLRepo "github.com/superj80820/system-design/auth/repository/auth/mysql"
	"github.com/superj80820/system-design/auth/usecase/account"
	"github.com/superj80820/system-design/auth/usecase/auth"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
)

func TestUseCase(t *testing.T) {
	ctx := context.Background()
	email := "email"
	password := "password"

	mySQLContainer, err := mysqlContainer.CreateMySQL(ctx)
	assert.Nil(t, err)
	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel, loggerKit.NoStdout)
	if err != nil {
		panic(err)
	}

	ormDB, err := ormKit.CreateDB(ormKit.UseMySQL(mySQLContainer.GetURI()))
	assert.Nil(t, err)

	accountSchema, err := os.ReadFile(filepath.Join("../repository/account/mysql", "schema.sql"))
	assert.Nil(t, err)
	assert.Nil(t, ormDB.Exec(string(accountSchema)).Error)
	authSchema, err := os.ReadFile(filepath.Join("../repository/auth/mysql", "schema.sql"))
	assert.Nil(t, err)
	assert.Nil(t, ormDB.Exec(string(authSchema)).Error)

	authRepo := authMySQLRepo.CreateAuthRepo(ormDB)
	accountRepo := accountMySQLRepo.CreateAccountRepo(ormDB)

	authUseCase, err := auth.CreateAuthUseCase("./test-access-private-key.pem", "./test-refresh-private-key.pem", authRepo, accountRepo, logger)
	assert.Nil(t, err)
	accountUseCase, err := account.CreateAccountUseCase(accountRepo, logger)
	assert.Nil(t, err)

	userInfo, err := accountUseCase.Register(email, password)
	assert.Nil(t, err)
	accountResult, err := authUseCase.Login(email, password)
	assert.Nil(t, err)
	userID, err := authUseCase.Verify(accountResult.AccessToken)
	assert.Nil(t, err)

	assert.Equal(t, userInfo.ID, userID)
}
