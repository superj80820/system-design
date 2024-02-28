package container

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

type kafkaContainer struct {
	uri       string
	container *kafka.KafkaContainer
}

func CreateKafka(ctx context.Context) (testing.KafkaContainer, error) {
	container, err := kafka.RunContainer(
		ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "run container failed")
	}
	kafkaHost, err := container.Host(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get container host failed")
	}
	kafkaPort, err := container.MappedPort(ctx, "9093") // TODO: is correct?
	if err != nil {
		return nil, errors.Wrap(err, "mapped container port failed")
	}

	return &kafkaContainer{
		uri:       fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port()),
		container: container,
	}, nil
}

func (k *kafkaContainer) GetURI() string {
	return k.uri
}

func (k *kafkaContainer) Terminate(ctx context.Context) error {
	if err := k.container.Terminate(ctx); err != nil {
		return errors.Wrap(err, "terminate failed")
	}
	return nil
}
