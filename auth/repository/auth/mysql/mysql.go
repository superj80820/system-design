package mysql

import (
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type accountTokenEntity struct {
	*domain.AccountToken
}

func (accountTokenEntity) TableName() string {
	return "account_token"
}

type authRepo struct {
	db *ormKit.DB
}

func CreateAuthRepo(db *ormKit.DB) domain.AuthRepo {
	return &authRepo{
		db: db,
	}
}

func (a *authRepo) CreateToken(userID int64, token string, expireAt time.Time, tokenType domain.AccountTokenEnum) (*domain.AccountToken, error) {
	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return nil, errors.Wrap(err, "generate unique id failed")
	}

	now := time.Now()

	tokenInstance := accountTokenEntity{
		AccountToken: &domain.AccountToken{
			ID:        uniqueIDGenerate.Generate().GetInt64(),
			Token:     token,
			Type:      domain.REFRESH_TOKEN,
			UserID:    userID,
			Status:    domain.ACTIVE,
			ExpireAt:  expireAt,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := a.db.Create(&tokenInstance).Error; err != nil {
		return nil, errors.Wrap(err, "save refresh token failed")
	}

	return tokenInstance.AccountToken, nil
}

func (a *authRepo) GetLastRefreshTokenByUserID(userID int64) (*domain.AccountToken, error) {
	var refreshToken accountTokenEntity
	if err := a.db.Last(&refreshToken, "user_id = ?", userID); err != nil { // TODO: check user_id name
		return nil, errors.Wrap(err, "get refresh token failed")
	}
	return refreshToken.AccountToken, nil
}

func (a *authRepo) UpdateStatusToken(token string, status domain.AccountTokenStatusEnum) error {
	if err := a.db.Model(&accountTokenEntity{}).Where("token = ?", token).Updates(accountTokenEntity{
		&domain.AccountToken{
			Status:    status,
			UpdatedAt: time.Now(),
		},
	}).Error; err != nil {
		return errors.Wrap(err, "update refresh token failed")
	}
	return nil
}
