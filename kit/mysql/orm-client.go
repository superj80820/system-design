package mysql

import (
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrRecordNotFound = gorm.ErrRecordNotFound // TODO: test found

type DB struct {
	gormClient *gorm.DB
}

type TX struct {
	*gorm.DB
}

func (tx *TX) IsCreate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return false
	}

	return tx.RowsAffected != 0
}

func CreateDB(dsn string) (*DB, error) {
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

func (db *DB) FirstOrCreate(dest interface{}, conds ...interface{}) TX {
	return TX{db.gormClient.FirstOrCreate(dest, conds...)}
}

func (db *DB) Model(value interface{}) TX {
	return TX{db.gormClient.Model(value)}
}

func (db *DB) Where(query interface{}, args ...interface{}) TX {
	return TX{db.gormClient.Where(query, args...)}
}

func (db *DB) Update(column string, value interface{}) TX {
	return TX{db.gormClient.Update(column, value)}
}

func (db *DB) Find(dest interface{}, conds ...interface{}) TX {
	return TX{db.gormClient.Find(dest, conds...)}
}

func (db *DB) Create(value interface{}) error {
	return db.gormClient.Create(value).Error
}

func (db *DB) First(dest interface{}, conds ...interface{}) error {
	return db.gormClient.First(dest, conds...).Error // TODO: check ...
}

func (db *DB) Last(dest interface{}, conds ...interface{}) error {
	return db.gormClient.Last(dest, conds...).Error // TODO: check ...
}

func (db *DB) Save(value interface{}) error {
	return db.gormClient.Save(value).Error // TODO: check ...
}
