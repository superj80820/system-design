package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func TestActressRepo(t *testing.T) {
	orm, err := ormKit.CreateDB(ormKit.UsePostgres("postgresql://postgres:postgres@0.0.0.0:5433/messfar"))
	assert.Nil(t, err)
	actressRepo := CreateActressRepo(orm)

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

	favorites, err := actressRepo.GetFavorites(userID)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(favorites))

	assert.Nil(t, actressRepo.RemoveFavorite(userID, actressID))

	favorites, err = actressRepo.GetFavorites(userID)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(favorites))

	actress, err = actressRepo.GetWish()
	assert.Nil(t, err)
	assert.NotEqual(t, "", actress.ID)
}
