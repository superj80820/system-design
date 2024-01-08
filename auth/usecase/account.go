package usecase

import (
	"net/http"

	"github.com/pkg/errors"

	"github.com/superj80820/system-design/auth/repository"
	"github.com/superj80820/system-design/kit/code"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type accountService struct {
	db     *ormKit.DB
	logger loggerKit.Logger
}

func CreateAccountService(db *ormKit.DB, logger loggerKit.Logger) (*accountService, error) {
	if db == nil || logger == nil {
		return nil, errors.New("create service failed")
	}
	return &accountService{
		db:     db,
		logger: logger,
	}, nil
}

func (a *accountService) Register(email, password string) error {
	// TODO: verify email and password format

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return errors.Wrap(err, "generate unique id failed")
	}

	err = a.db.FirstOrCreate(&repository.AccountEntity{
		ID:       uniqueIDGenerate.Generate().GetInt64(),
		Email:    email,
		Password: utilKit.GetSHA256(password),
	}, repository.AccountEntity{Email: email})
	if mysqlErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mysqlErr, ormKit.ErrDuplicatedKey) {
		return code.CreateErrorCode(http.StatusForbidden)
	} else if err != nil {
		return errors.Wrap(err, "create to db user failed")
	}

	return nil
}