package testing

import "context"

type MongoDBContainer interface {
	GetURI() string
	Terminate(context.Context) error
}

type RedisContainer interface {
	GetURI() string
	Terminate(context.Context) error
}

type MySQLContainer interface {
	GetURI() string
	Terminate(context.Context) error
}

type KafkaContainer interface {
	GetURI() string
	Terminate(context.Context) error
}

type PostgresContainer interface {
	GetURI() string
	Terminate(context.Context) error
}
