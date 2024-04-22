package line

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	authMySQLRepo "github.com/superj80820/system-design/auth/repository/auth/mysql"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
	lineRepo "github.com/superj80820/system-design/line/repository"
)

type mockLineLoginAPIRepo struct{}

func createMockLineLoginAPIRepo() domain.LineLoginAPIRepo {
	return &mockLineLoginAPIRepo{}
}

func (m *mockLineLoginAPIRepo) IssueAccessToken(code string, redirectURI string) (*domain.LineIssueAccessToken, error) {
	return &domain.LineIssueAccessToken{
		AccessToken:  "bNl4YEFPI/hjFWhTqexp4MuEw5YPs",
		ExpiresIn:    2592000,
		IDToken:      "eyJhbGciOiJIUzI1NiJ9",
		RefreshToken: "Aa1FdeggRhTnPNNpxr8p",
		Scope:        "profile",
		TokenType:    "Bearer",
	}, nil
}

func (m *mockLineLoginAPIRepo) VerifyToken(accessToken string) (*domain.LineVerifyToken, error) {
	return &domain.LineVerifyToken{
		Iss:     "https://access.line.me",
		Sub:     "U1234567890abcdef1234567890abcdef",
		Aud:     "1234567890",
		Exp:     1504169092,
		Iat:     1504263657,
		Nonce:   "0987654asdf",
		Amr:     []string{"pwd"},
		Name:    "Taro Line",
		Picture: "https://sample_line.me/aBcdefg123456",
		Email:   "taro.line@example.com",
	}, nil
}

func TestLine(t *testing.T) {
	ctx := context.Background()

	mysqlContainer, err := mysqlContainer.CreateMySQL(ctx, mysqlContainer.UseSQLSchema(
		filepath.Join("../../repository/account/mysql", "schema.sql"),
		filepath.Join("../../repository/auth/mysql", "schema.sql"),
		filepath.Join("../../../line/repository", "schema.sql"),
	))
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(ormKit.UseMySQL(mysqlContainer.GetURI()))
	assert.Nil(t, err)

	lineLoginAPIRepo := createMockLineLoginAPIRepo()
	lineUserRepo := lineRepo.CreateLineUserRepo(mysqlDB)
	accountRepo := accountMySQLRepo.CreateAccountRepo(mysqlDB)
	authRepo, err := authMySQLRepo.CreateAuthRepo(mysqlDB, "./access-private-key.pem", "./refresh-private-key.pem")
	assert.Nil(t, err)

	authLineUseCaseHandler := CreateAuthLineUseCase(accountRepo, lineLoginAPIRepo, lineUserRepo, authRepo)

	userInformation, lineUserProfile, err := authLineUseCaseHandler.VerifyCode("code", "http://redirect")
	assert.Nil(t, err)
	assert.NotNil(t, userInformation)
	assert.NotNil(t, lineUserProfile)
}
