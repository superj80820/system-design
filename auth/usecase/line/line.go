package line

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type authLineUseCase struct {
	accountRepo      domain.AccountRepo
	lineLoginAPIRepo domain.LineLoginAPIRepo
	lineUserRepo     domain.LineUserRepo
	authRepo         domain.AuthRepo
}

func CreateAuthLineUseCase(accountRepo domain.AccountRepo, lineLoginAPIRepo domain.LineLoginAPIRepo, lineUserRepo domain.LineUserRepo, authRepo domain.AuthRepo) domain.AuthLineUseCase {
	return &authLineUseCase{
		accountRepo:      accountRepo,
		lineLoginAPIRepo: lineLoginAPIRepo,
		lineUserRepo:     lineUserRepo,
		authRepo:         authRepo,
	}
}

func (a *authLineUseCase) VerifyCode(code, redirectURI string) (*domain.Account, *domain.LineUserProfile, error) {
	lineIssueAccessToken, err := a.lineLoginAPIRepo.IssueAccessToken(code, redirectURI)
	if err != nil {
		return nil, nil, errors.Wrap(err, "verify line code failed")
	}
	lineVerifyToken, err := a.lineLoginAPIRepo.VerifyToken(lineIssueAccessToken.IDToken)
	if err != nil {
		return nil, nil, errors.Wrap(err, "verify line code failed")
	}
	var userID int64
	lineUserProfile, err := a.lineUserRepo.Get(lineVerifyToken.Sub)
	if errors.Is(err, domain.ErrNoData) {
		userInformation, err := a.accountRepo.Create(utilKit.GetSnowflakeIDString(), utilKit.GetSnowflakeIDString()) // TODO: utilKit.GetSnowflakeIDString() workaround
		if err != nil {
			return nil, nil, errors.Wrap(err, "create user failed")
		}
		userID = userInformation.ID
		lineUserProfile, err = a.lineUserRepo.Create(userInformation.ID, lineVerifyToken.Sub, lineVerifyToken.Name, lineVerifyToken.Picture)
		if err != nil {
			return nil, nil, errors.Wrap(err, "create line user profile failed")
		}
	} else if err != nil {
		return nil, nil, errors.Wrap(err, "verify line code failed")
	} else {
		userID = lineUserProfile.UserID
	}

	now := time.Now()

	refreshTokenExpireAt := now.Add(time.Hour * 12)
	signedRefreshToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(userID, 10),
		now,
		refreshTokenExpireAt,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "signed refresh token failed")
	}

	accessTokenExpireAt := now.Add(time.Hour * 1)
	signedAccessToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(userID, 10),
		now,
		accessTokenExpireAt,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "signed refresh token failed")
	}

	_, err = a.authRepo.CreateToken(userID, signedRefreshToken, refreshTokenExpireAt, domain.REFRESH_TOKEN)
	if err != nil {
		return nil, nil, errors.Wrap(err, "save refresh token failed")
	}

	return &domain.Account{
		ID:           userID,
		Email:        lineVerifyToken.Email,
		AccessToken:  signedAccessToken,
		RefreshToken: signedRefreshToken,
	}, lineUserProfile, nil
}

func (a *authLineUseCase) VerifyLIFFToken(token string) (*domain.Account, *domain.LineUserProfile, error) {
	lineVerifyToken, err := a.lineLoginAPIRepo.VerifyToken(token)
	if err != nil {
		return nil, nil, errors.Wrap(err, "verify liff token failed")
	}
	var userID int64
	lineUserProfile, err := a.lineUserRepo.Get(lineVerifyToken.Sub)
	if errors.Is(err, domain.ErrNoData) {
		userInformation, err := a.accountRepo.Create(utilKit.GetSnowflakeIDString(), utilKit.GetSnowflakeIDString()) // TODO: utilKit.GetSnowflakeIDString() workaround
		if err != nil {
			return nil, nil, errors.Wrap(err, "create user failed")
		}
		userID = userInformation.ID
		lineUserProfile, err = a.lineUserRepo.Create(userInformation.ID, lineVerifyToken.Sub, lineVerifyToken.Name, lineVerifyToken.Picture)
		if err != nil {
			return nil, nil, errors.Wrap(err, "create line user profile failed")
		}
	} else if err != nil {
		return nil, nil, errors.Wrap(err, "verify liff token failed")
	} else {
		userID = lineUserProfile.UserID
	}

	now := time.Now()

	refreshTokenExpireAt := now.Add(time.Hour * 12)
	signedRefreshToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(userID, 10),
		now,
		refreshTokenExpireAt,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "signed refresh token failed")
	}

	accessTokenExpireAt := now.Add(time.Hour * 1)
	signedAccessToken, err := a.authRepo.GenerateToken(
		strconv.FormatInt(userID, 10),
		now,
		accessTokenExpireAt,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "signed refresh token failed")
	}

	_, err = a.authRepo.CreateToken(userID, signedRefreshToken, refreshTokenExpireAt, domain.REFRESH_TOKEN)
	if err != nil {
		return nil, nil, errors.Wrap(err, "save refresh token failed")
	}

	return &domain.Account{
		ID:           userID,
		Email:        lineVerifyToken.Email,
		AccessToken:  signedAccessToken,
		RefreshToken: signedRefreshToken,
	}, lineUserProfile, nil
}
