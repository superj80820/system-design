package container

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

type mysqlContainer struct {
	uri       string
	container *mysql.MySQLContainer
}

func CreateMySQL(ctx context.Context, sqlSchemaPath string) (testing.MySQLContainer, error) {
	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"
	container, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
		mysql.WithScripts(sqlSchemaPath),
	)
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

	return &mysqlContainer{
		uri: fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			mysqlDBUsername,
			mysqlDBPassword,
			mysqlDBHost,
			mysqlDBPort.Port(),
			mysqlDBName,
		),
		container: container,
	}, nil
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
