package memoryandmysql

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	testingMySQLKit "github.com/superj80820/system-design/kit/testing/mysql/container"
)

func TestUserAssetRepo(t *testing.T) {
	ctx := context.Background()

	mysql, err := testingMySQLKit.CreateMySQL(ctx, testingMySQLKit.UseSQLSchema("./schema.sql"))
	assert.Nil(t, err)
	assetMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100, 100*time.Millisecond)

	orm, err := ormKit.CreateDB(ormKit.UseMySQL(mysql.GetURI()))
	assert.Nil(t, err)

	assetRepo := CreateAssetRepo(assetMQTopic, orm)

	// should update
	{
		timeNow := time.Now()
		assert.Nil(t, assetRepo.SaveAssets(1, []*domain.UserAsset{
			{
				ID:        1,
				UserID:    2,
				AssetID:   1,
				Available: decimal.NewFromInt(100),
				Frozen:    decimal.NewFromInt(100),
				CreatedAt: timeNow,
				UpdatedAt: timeNow,
			},
			{
				ID:        2,
				UserID:    2,
				AssetID:   2,
				Available: decimal.NewFromInt(1000),
				Frozen:    decimal.NewFromInt(1000),
				CreatedAt: timeNow,
				UpdatedAt: timeNow,
			},
		}))

		usersAssets, err := assetRepo.GetHistoryAssets(2)
		assert.Nil(t, err)

		assert.Equal(t, 2, len(usersAssets))
		assert.Equal(t, "100", usersAssets[1].Available.String())
		assert.Equal(t, "100", usersAssets[1].Frozen.String())
		assert.Equal(t, "1000", usersAssets[2].Available.String())
		assert.Equal(t, "1000", usersAssets[2].Frozen.String())
	}

	// should update
	{
		timeNow := time.Now()
		assert.Nil(t, assetRepo.SaveAssets(2, []*domain.UserAsset{
			{
				ID:        1,
				UserID:    2,
				AssetID:   1,
				Available: decimal.NewFromInt(101),
				Frozen:    decimal.NewFromInt(101),
				CreatedAt: timeNow,
				UpdatedAt: timeNow,
			},
			{
				ID:        2,
				UserID:    2,
				AssetID:   2,
				Available: decimal.NewFromInt(1001),
				Frozen:    decimal.NewFromInt(1001),
				CreatedAt: timeNow,
				UpdatedAt: timeNow,
			},
		}))

		usersAssets, err := assetRepo.GetHistoryAssets(2)
		assert.Nil(t, err)

		assert.Equal(t, 2, len(usersAssets))
		assert.Equal(t, "101", usersAssets[1].Available.String())
		assert.Equal(t, "101", usersAssets[1].Frozen.String())
		assert.Equal(t, "1001", usersAssets[2].Available.String())
		assert.Equal(t, "1001", usersAssets[2].Frozen.String())
	}

	// asset id 2 should not update, because sequence id is before
	{
		timeNow := time.Now()
		assert.Nil(t, assetRepo.SaveAssets(3, []*domain.UserAsset{
			{
				ID:        1,
				UserID:    2,
				AssetID:   1,
				Available: decimal.NewFromInt(102),
				Frozen:    decimal.NewFromInt(102),
				CreatedAt: timeNow,
				UpdatedAt: timeNow,
			},
		}))
		assert.Nil(t, assetRepo.SaveAssets(2, []*domain.UserAsset{
			{
				ID:        2,
				UserID:    2,
				AssetID:   2,
				Available: decimal.NewFromInt(1002),
				Frozen:    decimal.NewFromInt(1002),
				CreatedAt: timeNow,
				UpdatedAt: timeNow,
			},
		}))

		usersAssets, err := assetRepo.GetHistoryAssets(2)
		assert.Nil(t, err)

		assert.Equal(t, 2, len(usersAssets))
		assert.Equal(t, "102", usersAssets[1].Available.String())
		assert.Equal(t, "102", usersAssets[1].Frozen.String())
		assert.Equal(t, "1001", usersAssets[2].Available.String())
		assert.Equal(t, "1001", usersAssets[2].Frozen.String())
	}
}
