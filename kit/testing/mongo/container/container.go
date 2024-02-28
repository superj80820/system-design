package container

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
)

type mongodbContainer struct {
	uri       string
	container *mongodb.MongoDBContainer
}

func CreateMongoDB(ctx context.Context) (testing.MongoDBContainer, error) {
	container, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	if err != nil {
		return nil, errors.Wrap(err, "run container failed")
	}
	mongoHost, err := container.Host(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get container host failed")
	}
	mongoPort, err := container.MappedPort(ctx, "27017")
	if err != nil {
		return nil, errors.Wrap(err, "mapped container port failed")
	}

	return &mongodbContainer{
		uri:       "mongodb://" + mongoHost + ":" + mongoPort.Port(),
		container: container,
	}, nil
}

func (m *mongodbContainer) GetURI() string {
	return m.uri
}

func (m *mongodbContainer) Terminate(ctx context.Context) error {
	if err := m.container.Terminate(ctx); err != nil {
		return errors.Wrap(err, "terminate failed")
	}
	return nil
}
