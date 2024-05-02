package domain

type FaceSearch struct {
	SearchResults []*FaceSearchResults `json:"results"`
}

type FaceSearchResults struct {
	Confidence float64 `json:"confidence"`
	UserID     string  `json:"user_id"`
	FaceToken  string  `json:"face_token"`
}

type FaceDetect struct {
	RequestID string `json:"request_id"`
	TimeUsed  int    `json:"time_used"`
	Faces     []struct {
		FaceToken     string `json:"face_token"`
		FaceRectangle struct {
			Top    int `json:"top"`
			Left   int `json:"left"`
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"face_rectangle"`
	} `json:"faces"`
	ImageID string `json:"image_id"`
	FaceNum int    `json:"face_num"`
}

type FaceSetDetail struct {
	FacesetToken string   `json:"faceset_token"`
	Tags         string   `json:"tags"`
	TimeUsed     int      `json:"time_used"`
	UserData     string   `json:"user_data"`
	DisplayName  string   `json:"display_name"`
	FaceTokens   []string `json:"face_tokens"`
	FaceCount    int      `json:"face_count"`
	RequestID    string   `json:"request_id"`
	OuterID      string   `json:"outer_id"`
}

type FaceAdd struct {
	FacesetToken  string           `json:"faceset_token"`
	TimeUsed      int              `json:"time_used"`
	FaceCount     int              `json:"face_count"`
	FaceAdded     int              `json:"face_added"`
	RequestID     string           `json:"request_id"`
	OuterID       string           `json:"outer_id"`
	FailureDetail []map[string]any `json:"failure_detail"`
}

type FacePlusPlusRepo interface {
	Search(faceSetID string, image []byte) (*FaceSearch, error)
	Add(faceSetID string, faceTokens []string) (*FaceAdd, error)
	Detect(image []byte) (*FaceDetect, error)
	GetFaceSetDetail(faceSetID string) (*FaceSetDetail, error)
	IsFaceSetFull(faceSetID string) (bool, error)
}

type FacePlusPlusWorkPoolUseCase interface {
	Search(faceSetID string, image []byte) (*FaceSearch, error)
	Add(faceSetID string, faceTokens []string) (*FaceAdd, error)
	Detect(image []byte) (*FaceDetect, error)
	GetFaceSetDetail(faceSetID string) (*FaceSetDetail, error)
	IsFaceSetFull(faceSetID string) (bool, error)
}

type FacePlusPlusUseCase interface {
	SearchAllFaceSets(image []byte) (*FaceSearch, error)
	Add(faceTokens []string) (faceSetToken string, err error)
	Detect(image []byte) (*FaceDetect, error)
}
