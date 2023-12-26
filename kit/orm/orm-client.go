package orm

import (
	"database/sql"

	goMysql "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrRecordNotFound = gorm.ErrRecordNotFound // TODO: test found
	ErrDuplicatedKey  = gorm.ErrDuplicatedKey
)

type mySQLConfig struct {
	dns string
}

type sqliteConfig struct {
	fileName string
}

type DB struct {
	gormClient *gorm.DB

	dbType dbType

	mySQLConfig  *mySQLConfig
	sqliteConfig *sqliteConfig
}

type TX = gorm.DB

type dbType int

const (
	dbTypeNoop dbType = iota
	dbTypeMySQL
	dbTypeSQLite
)

type Option func(*DB)

func UseMySQL(dns string) Option {
	return func(db *DB) {
		db.mySQLConfig = &mySQLConfig{
			dns: dns,
		}
	}
}

func UseSQLite(fileName string) Option {
	return func(db *DB) {
		db.dbType = dbTypeSQLite
		db.sqliteConfig = &sqliteConfig{
			fileName: fileName,
		}
	}
}

func UseNoop(db *DB) {
	db.dbType = dbTypeNoop
}

func CreateDB(useDB Option, options ...Option) (*DB, error) {
	var gormDB DB

	useDB(&gormDB)
	for _, option := range options {
		option(&gormDB)
	}

	if gormDB.dbType == dbTypeNoop {
		return &gormDB, nil
	}

	var dialector gorm.Dialector
	switch gormDB.dbType {
	case dbTypeMySQL:
		dialector = mysql.Open(gormDB.mySQLConfig.dns)
	case dbTypeSQLite:
		dialector = sqlite.Open(gormDB.sqliteConfig.fileName)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
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

	gormDB.gormClient = db

	return &gormDB, nil
}

func (db *DB) Exec(sql string, values ...interface{}) *TX {
	return db.gormClient.Exec(sql, values...)
}

func (db *DB) Transaction(fc func(tx *gorm.DB) error, opts ...*sql.TxOptions) (err error) {
	return db.gormClient.Transaction(fc, opts...)
}

func (db *DB) FirstOrCreate(dest interface{}, conds ...interface{}) error {
	return db.gormClient.FirstOrCreate(dest, conds...).Error
}

func (db *DB) Model(value interface{}) *TX {
	return db.gormClient.Model(value)
}

func (db *DB) Debug() *TX {
	return db.gormClient.Debug()
}

func (db *DB) Where(query interface{}, args ...interface{}) *TX {
	return db.gormClient.Where(query, args...)
}

func (db *DB) Update(column string, value interface{}) *TX {
	return db.gormClient.Update(column, value)
}

func (db *DB) Find(dest interface{}, conds ...interface{}) *TX {
	return db.gormClient.Find(dest, conds...)
}

func (db *DB) Create(value interface{}) *TX {
	return db.gormClient.Create(value)
}

func (db *DB) First(dest interface{}, conds ...interface{}) error {
	return db.gormClient.First(dest, conds...).Error // TODO: check ...
}

func (db *DB) Last(dest interface{}, conds ...interface{}) error {
	return db.gormClient.Last(dest, conds...).Error // TODO: check ...
}

func (db *DB) Save(value interface{}) *TX {
	return db.gormClient.Save(value)
}

func ConvertMySQLErr(err error) (error, bool) {
	var mysqlErr *goMysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrDuplicatedKey, true
	}
	return nil, false
}
