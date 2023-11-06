package mysql

import (
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	gormClient *gorm.DB
}

func CreateSingletonDB(dsn string) (*DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, errors.Wrap(err, "connect db failed")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "get core db failed")
	}
	if sqlDB.Ping() != nil {
		return nil, errors.Wrap(err, "ping core db failed")
	}
	return &DB{
		gormClient: db,
	}, nil
}

func (db *DB) Create(value interface{}) error {
	return db.gormClient.Create(value).Error
}

func (db *DB) First(dest interface{}, conds ...interface{}) error {
	return db.gormClient.First(dest, conds...).Error // TODO: check ...
}
