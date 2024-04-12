package repository

import (
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type accountEntity struct {
	domain.Account
}

func (accountEntity) TableName() string {
	return "account"
}

type accountRepo struct {
	db *ormKit.DB
}

func CreateAccountRepo(db *ormKit.DB) domain.AccountRepo {
	return &accountRepo{
		db: db,
	}
}

func (a *accountRepo) Create(email string, password string) (*domain.Account, error) {
	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return nil, errors.Wrap(err, "generate unique id failed")
	}

	hash, err := utilKit.GetBcrypt(password)
	if err != nil {
		return nil, errors.Wrap(err, "get bcrypt failed")
	}

	account := accountEntity{
		Account: domain.Account{
			ID:       uniqueIDGenerate.Generate().GetInt64(),
			Email:    email,
			Password: hash,
		},
	}

	if err = a.db.Create(&account).Error; err != nil {
		return nil, errors.Wrap(err, "create failed")
	}

	return &account.Account, nil
}

func (a *accountRepo) Get(userID int) (*domain.Account, error) {
	var account accountEntity
	if err := a.db.First(&account, userID); err != nil {
		return nil, errors.Wrap(err, "get account failed")
	}
	return &account.Account, nil
}

func (a *accountRepo) GetEmail(email string) (*domain.Account, error) {
	var account accountEntity
	if err := a.db.First(&account, "email = ?", email); err != nil {
		return nil, errors.Wrap(err, "get account failed")
	}
	return &account.Account, nil
}
