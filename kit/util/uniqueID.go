package util

import (
	"github.com/bwmarrin/snowflake"
	"github.com/jxskiss/base62"
	"github.com/pkg/errors"
)

type UniqueIDGenerate struct {
	snowflakeNode *snowflake.Node
}

var singletonUniqueIDGenerate *UniqueIDGenerate

func GetUniqueIDGenerate() (*UniqueIDGenerate, error) { // TODO: don't use lazy, use DI?
	if singletonUniqueIDGenerate != nil {
		return singletonUniqueIDGenerate, nil // TODO: york
	}
	snowflakeNode, err := snowflake.NewNode(1) // TODO: york
	if err != nil {
		return nil, errors.Wrap(err, "create snowflake failed")
	}
	singletonUniqueIDGenerate = &UniqueIDGenerate{
		snowflakeNode: snowflakeNode,
	}
	return singletonUniqueIDGenerate, nil
}

func (u UniqueIDGenerate) Generate() *UniqueID {
	return &UniqueID{
		snowflakeID: u.snowflakeNode.Generate(),
	}
}

type UniqueID struct {
	snowflakeID snowflake.ID
}

func (u UniqueID) GetInt64() int64 {
	return u.snowflakeID.Int64() // TODO: york unit test
}

func (u UniqueID) GetBase62() string {
	return string(base62.FormatInt(u.snowflakeID.Int64())) // TODO: york unit test
}

var singletonSnowflakeNode *snowflake.Node

func init() {
	snowflakeNode, err := snowflake.NewNode(1) // TODO: york
	if err != nil {
		panic(errors.Wrap(err, "create snowflake failed"))
	}
	singletonSnowflakeNode = snowflakeNode
}

func GetSnowflakeIDInt64() int64 {
	return singletonSnowflakeNode.Generate().Int64()
}

func GetSnowflakeIDString() string {
	return singletonSnowflakeNode.Generate().String()
}
