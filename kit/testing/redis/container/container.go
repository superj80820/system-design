package container

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

type redisContainer struct {
	uri       string
	container *redis.RedisContainer
}

func CreateRedis(ctx context.Context) (testing.RedisContainer, error) {
	container, err := redis.RunContainer(ctx,
		testcontainers.WithImage("docker.io/redis:7"),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		return nil, errors.Wrap(err, "run container failed")
	}
	redisHost, err := container.Host(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get container host failed")
	}
	redisPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return nil, errors.Wrap(err, "mapped container port failed")
	}

	return &redisContainer{
		container: container,
		uri:       redisHost + ":" + redisPort.Port(),
	}, nil
}

func (r *redisContainer) GetURI() string {
	return r.uri
}

func (r *redisContainer) Terminate(ctx context.Context) error {
	if err := r.container.Terminate(ctx); err != nil {
		return errors.Wrap(err, "terminate failed")
	}
	return nil
}
