package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type AuthService struct {
	authRepo    domain.AuthRepo
	accountRepo domain.AccountRepo
	logger      loggerKit.Logger
}

func CreateAuthUseCase(authRepo domain.AuthRepo, accountRepo domain.AccountRepo, logger loggerKit.Logger) (domain.AuthUseCase, error) {
	if logger == nil {
		return nil, errors.New("create service failed")
	}
	return &AuthService{
		logger:      logger,
		authRepo:    authRepo,
		accountRepo: accountRepo,
	}, nil
}

func (a *AuthService) Login(email, password string) (*domain.Account, error) {
	account, err := a.accountRepo.GetEmail(email)
	if errors.Is(err, ormKit.ErrRecordNotFound) {
		return nil, code.CreateErrorCode(http.StatusUnauthorized)
	} else if err != nil {
		return nil, errors.Wrap(err, "get db user failed")
	}

	if err := utilKit.CompareBcrypt([]byte(account.Password), []byte(password)); err != nil {
		return nil, code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.PasswordInvalid)
	}

	now := time.Now()

	refreshTokenExpireAt := now.Add(time.Hour * 12)
	signedRefreshToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(account.ID, 10),
		now,
		refreshTokenExpireAt,
	)
	if err != nil {
		return nil, errors.Wrap(err, "signed refresh token failed")
	}

	accessTokenExpireAt := now.Add(time.Hour * 1)
	signedAccessToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(account.ID, 10),
		now,
		accessTokenExpireAt,
	)
	if err != nil {
		return nil, errors.Wrap(err, "signed refresh token failed")
	}

	_, err = a.authRepo.CreateToken(account.ID, signedRefreshToken, refreshTokenExpireAt, domain.REFRESH_TOKEN)
	if err != nil {
		return nil, errors.Wrap(err, "save refresh token failed")
	}

	return &domain.Account{
		Email:        email,
		AccessToken:  signedAccessToken,
		RefreshToken: signedRefreshToken,
	}, nil
}

func (a *AuthService) Logout(accessToken string) error {
	userID, err := a.authRepo.VerifyToken(accessToken, domain.ACCESS_TOKEN)
	if errors.Is(err, domain.ErrInvalidData) {
		return code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.PasswordInvalid).AddErrorMetaData(err)
	} else if errors.Is(err, domain.ErrExpired) {
		return code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Expired).AddErrorMetaData(err)
	} else if err != nil {
		return errors.Wrap(err, "verify token failed")
	}

	refreshToken, err := a.authRepo.GetLastRefreshTokenByUserID(userID)
	if err != nil {
		return errors.Wrap(err, "get refresh token failed")
	}

	if err = a.authRepo.UpdateStatusToken(refreshToken.Token, domain.REVOKE); err != nil {
		return errors.Wrap(err, "update refresh token failed")
	}

	return nil
}

func (a *AuthService) RefreshAccessToken(refreshTokenString string) (string, error) {
	userID, err := a.authRepo.VerifyToken(refreshTokenString, domain.REFRESH_TOKEN)
	if errors.Is(err, domain.ErrInvalidData) {
		return "", code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.PasswordInvalid).AddErrorMetaData(err)
	} else if errors.Is(err, domain.ErrExpired) {
		return "", code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Expired).AddErrorMetaData(err)
	} else if err != nil {
		return "", errors.Wrap(err, "verify token failed")
	}

	refreshToken, err := a.authRepo.GetLastRefreshTokenByUserID(userID)
	if err != nil {
		return "", errors.Wrap(err, "get refresh token failed")
	}

	if time.Now().After(refreshToken.ExpireAt) {
		return "", code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Expired)
	}

	if refreshToken.Status == domain.REVOKE {
		return "", code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Revoke)
	}

	now := time.Now()

	accessTokenExpireAt := now.Add(time.Hour * 1)
	signedAccessToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(userID, 10),
		now,
		accessTokenExpireAt,
	)
	if err != nil {
		return "", errors.Wrap(err, "signed refresh token failed")
	}

	return signedAccessToken, nil
}

func (a *AuthService) Verify(accessToken string) (int64, error) {
	userID, err := a.authRepo.VerifyToken(accessToken, domain.ACCESS_TOKEN)
	if errors.Is(err, domain.ErrInvalidData) {
		return 0, code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.PasswordInvalid).AddErrorMetaData(err)
	} else if errors.Is(err, domain.ErrExpired) {
		return 0, code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Expired).AddErrorMetaData(err)
	} else if err != nil {
		return 0, errors.Wrap(err, "verify token failed")
	}
	return userID, nil
}
