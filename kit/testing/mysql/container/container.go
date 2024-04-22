package container

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
	utilKit "github.com/superj80820/system-design/kit/util"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

type mysqlContainer struct {
	uri            string
	sqlSchemaPaths []string
	container      *mysql.MySQLContainer
}

type Option func(*mysqlContainer)

func UseSQLSchema(sqlSchemaPaths ...string) Option {
	return func(mc *mysqlContainer) {
		mc.sqlSchemaPaths = sqlSchemaPaths
	}
}

func CreateMySQL(ctx context.Context, options ...Option) (testing.MySQLContainer, error) {
	var mysqlContainer mysqlContainer
	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"

	for _, option := range options {
		option(&mysqlContainer)
	}

	runContainerOptions := []testcontainers.ContainerCustomizer{
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
	}

	if len(mysqlContainer.sqlSchemaPaths) != 0 {
		tempSQLSchemaFileName := "schema-" + utilKit.GetSnowflakeIDString() + ".sql"
		tempSQLSchemaFilePath := "./" + tempSQLSchemaFileName
		defer os.Remove(tempSQLSchemaFilePath)

		var tempSQLSchema bytes.Buffer
		for _, sqlSchemaPath := range mysqlContainer.sqlSchemaPaths {
			schemaSQL, err := os.ReadFile(sqlSchemaPath)
			if err != nil {
				return nil, errors.Wrap(err, "read file failed")
			}
			tempSQLSchema.Write([]byte(schemaSQL))
		}
		if err := os.WriteFile(tempSQLSchemaFilePath,
			tempSQLSchema.Bytes(),
			0644,
		); err != nil {
			return nil, errors.Wrap(err, "writer sql file failed")
		}

		runContainerOptions = append(runContainerOptions, mysql.WithScripts(tempSQLSchemaFilePath))
	}

	container, err := mysql.RunContainer(ctx, runContainerOptions...)
	if err != nil {
		return nil, errors.Wrap(err, "run container failed")
	}
	mysqlDBHost, err := container.Host(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get container host failed")
	}
	mysqlDBPort, err := container.MappedPort(ctx, "3306")
	if err != nil {
		return nil, errors.Wrap(err, "mapped container port failed")
	}

	mysqlContainer.uri = fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlDBUsername,
		mysqlDBPassword,
		mysqlDBHost,
		mysqlDBPort.Port(),
		mysqlDBName,
	)
	mysqlContainer.container = container

	return &mysqlContainer, nil
}

func (m *mysqlContainer) GetURI() string {
	return m.uri
}

func (m *mysqlContainer) Terminate(ctx context.Context) error {
	if err := m.container.Terminate(ctx); err != nil {
		return errors.Wrap(err, "terminate failed")
	}
	return nil
}
