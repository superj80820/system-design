package domain

type FaceSearchInformation struct {
	SearchResults []struct {
		Confidence float64 `json:"confidence"`
		UserID     string  `json:"user_id"`
		FaceToken  string  `json:"face_token"`
	} `json:"results"`
}

type FacePlusPlusRepo interface {
	Search(faceSetID string, image []byte) (*FaceSearchInformation, error)
	Add()
	Detect()
	GetFaceSetDetail()
}

type FacePlusPlusUseCase interface {
	SearchAllFaceSets(image []byte) (*FaceSearchInformation, error)
	Add()
	Detect()
	GetSetDetail()
}
