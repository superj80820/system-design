package facepp

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
)

func TestFacePlusPlusRepo(t *testing.T) {
	facePlusPlusRepo := CreateFacePlusPlusRepo(
		"https://api-cn.faceplusplus.com",
		"5R8V9KC-aPK-DNd_3Cydt141a6NVjCam",
		"45SqRizjSXQ-9sZZz2b1unuqSNFd1PWA",
		UseProxyURL("http://121.40.110.105:6080"),
	)

	faceImage, err := os.ReadFile("./test_face.jpg")
	assert.Nil(t, err)

	faceSetDetail, err := facePlusPlusRepo.GetFaceSetDetail("6f1318a444b00ac89265a680927d1452")
	assert.Nil(t, err)
	assert.Equal(t, "6f1318a444b00ac89265a680927d1452", faceSetDetail.FacesetToken)

	faceDetect, err := facePlusPlusRepo.Detect(faceImage)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(faceDetect.Faces))
	assert.NotEqual(t, "", faceDetect.Faces[0].FaceToken)

	faceAdd, err := facePlusPlusRepo.Add("6f1318a444b00ac89265a680927d1452", []string{faceDetect.Faces[0].FaceToken})
	assert.Nil(t, err)
	assert.Equal(t, 1, faceAdd.FaceAdded)

	faceAdd, err = facePlusPlusRepo.Add("6f1318a444b00ac89265a680927d1452", []string{faceDetect.Faces[0].FaceToken})
	assert.ErrorIs(t, err, domain.ErrAlreadyDone)
	assert.Nil(t, faceAdd)

	faceAdd, err = facePlusPlusRepo.Add("6f1318a444b00ac89265a680927d1452", []string{"6f1318a444b00ac89265a68092712345"})
	assert.ErrorIs(t, err, domain.ErrInvalidData)
	assert.Nil(t, faceAdd)

	faceSearch, err := facePlusPlusRepo.Search("6f1318a444b00ac89265a680927d1452", faceImage)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(faceSearch.SearchResults))
	assert.NotEqual(t, "", faceSearch.SearchResults[0].FaceToken)
}
