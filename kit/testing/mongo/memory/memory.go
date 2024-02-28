package memory

import (
	"context"
	"strconv"

	"github.com/benweissmann/memongo"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/testing"
)

type mongodbContainer struct {
	uri string
}

func CreateMongoDB() (testing.MongoDBContainer, error) {
	mongoServer, err := memongo.Start("4.0.5")
	if err != nil {
		return nil, errors.Wrap(err, "start failed")
	}

	return &mongodbContainer{
		uri: "mongodb://" + mongoServer.URI() + ":" + strconv.Itoa(mongoServer.Port()),
	}, nil
}

func (m *mongodbContainer) GetURI() string {
	return m.uri
}

// Terminate noop
func (m *mongodbContainer) Terminate(context.Context) error {
	return nil
}
