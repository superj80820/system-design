package usecase

import (
	"net/http"

	"github.com/pkg/errors"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
)

type accountService struct {
	accountRepo domain.AccountRepo
	logger      loggerKit.Logger
}

func CreateAccountUseCase(accountRepo domain.AccountRepo, logger loggerKit.Logger) (domain.AccountService, error) {
	if logger == nil {
		return nil, errors.New("create service failed")
	}
	return &accountService{
		accountRepo: accountRepo,
		logger:      logger,
	}, nil
}

func (a *accountService) Register(email, password string) (*domain.Account, error) {
	// TODO: verify email and password format

	account, err := a.accountRepo.Create(email, password)
	if mysqlErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mysqlErr, ormKit.ErrDuplicatedKey) {
		return nil, code.CreateErrorCode(http.StatusForbidden)
	} else if err != nil {
		return nil, errors.Wrap(err, "create to db user failed")
	}
	return account, nil
}

func (a *accountService) Get(userID int) (*domain.Account, error) {
	account, err := a.accountRepo.Get(userID)
	if err != nil {
		return nil, errors.Wrap(err, "get account failed")
	}
	return account, nil
}
