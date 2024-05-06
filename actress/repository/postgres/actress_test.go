package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	ormKit "github.com/superj80820/system-design/kit/orm"
	testingPostgresKit "github.com/superj80820/system-design/kit/testing/postgres/container"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func TestActressRepo(t *testing.T) {
	ctx := context.Background()

	postgres, err := testingPostgresKit.CreatePostgres(ctx, "./schema.sql")
	assert.Nil(t, err)

	orm, err := ormKit.CreateDB(ormKit.UsePostgres(postgres.GetURI()))
	assert.Nil(t, err)
	actressRepo, err := CreateActressRepo(orm)
	assert.Nil(t, err)

	actressID, err := actressRepo.AddActress("name_"+utilKit.GetSnowflakeIDString(), "preview")
	assert.Nil(t, err)
	assert.NotEqual(t, "", actressID)

	actress, err := actressRepo.GetActress(actressID)
	assert.Nil(t, err)
	assert.Equal(t, actress.ID, actressID)

	faceToken := utilKit.GetSnowflakeIDString()
	previewURL := "https://i.imgur.com/RH7MCl1.jpg"
	faceID, err := actressRepo.AddFace(actressID, faceToken, previewURL)
	assert.Nil(t, err)
	assert.NotEqual(t, "", faceID)

	actressByFaceToken, err := actressRepo.GetActressByFaceToken(faceToken)
	assert.Nil(t, err)
	assert.Equal(t, actressID, actressByFaceToken.ID)
	assert.Equal(t, "preview", actressByFaceToken.Preview)

	assert.Nil(t, actressRepo.SetActressPreview(actressID, ""))

	actress, err = actressRepo.GetActress(actressID)
	assert.Nil(t, err)
	assert.Equal(t, "", actress.Preview)

	userID := utilKit.GetSnowflakeIDString()
	assert.Nil(t, actressRepo.AddFavorite(userID, actressID))

	favorite, err := actressRepo.GetFavoritesPagination(ctx, userID, 1, 10)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(favorite.Actresses))
	assert.True(t, favorite.IsEnd)
	assert.Equal(t, uint(1), favorite.Total)

	assert.Nil(t, actressRepo.RemoveFavorite(userID, actressID))

	favorite, err = actressRepo.GetFavoritesPagination(ctx, userID, 1, 10)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(favorite.Actresses))
	assert.True(t, favorite.IsEnd)
	assert.Equal(t, uint(0), favorite.Total)

	actress, err = actressRepo.GetWish()
	assert.Nil(t, err)
	assert.NotEqual(t, "", actress.ID)
}
