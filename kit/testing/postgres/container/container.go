package container

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type postgresContainer struct {
	uri       string
	container *postgres.PostgresContainer
}

func (p *postgresContainer) GetURI() string {
	return p.uri
}

func (p *postgresContainer) Terminate(ctx context.Context) error {
	if err := p.container.Terminate(ctx); err != nil {
		return errors.Wrap(err, "terminate failed")
	}
	return nil
}

type Option func(*postgresConfig)

type postgresConfig struct {
	zone           string
	configFilePath string
}

func SetZone(zone string) Option {
	return func(pc *postgresConfig) {
		pc.zone = zone
	}
}

func SetConfigFilePath(configFilePath string) Option {
	return func(pc *postgresConfig) {
		pc.configFilePath = configFilePath
	}
}

func CreatePostgres(ctx context.Context, scriptFilePath string, options ...Option) (testing.PostgresContainer, error) {
	dbName := "users"
	dbUser := "user"
	dbPassword := "password"

	config := &postgresConfig{
		zone: "Asia/Shanghai",
	}

	for _, option := range options {
		option(config)
	}

	containerOptions := []testcontainers.ContainerCustomizer{
		testcontainers.WithImage("docker.io/postgres:15.2-alpine"),
		postgres.WithInitScripts(scriptFilePath),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5 * time.Second)),
	}
	if config.configFilePath != "" {
		containerOptions = append(containerOptions, postgres.WithConfigFile(config.configFilePath))
	}

	container, err := postgres.RunContainer(ctx, containerOptions...)
	if err != nil {
		return nil, errors.Wrap(err, "run container failed")
	}

	dbHost, err := container.Host(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get container host failed")
	}
	dbPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, errors.Wrap(err, "mapped container port failed")
	}

	return &postgresContainer{
		uri: fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
			dbHost,
			dbUser,
			dbPassword,
			dbName,
			dbPort.Port(),
			config.zone,
		),
		container: container,
	}, nil
}
