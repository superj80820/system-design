package domain

type Actress struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Preview      string `json:"preview"`
	Detail       string `json:"detail"`
	Romanization string `json:"romanization"`
}

type ActressLineRepository interface {
	GetUseInformation() (string, error)
	IsEnableGroupRecognition(groupID string) (bool, error)
	EnableGroupRecognition(groupID string) error
	DisableGroupRecognition()
}

type ActressRepository interface {
	GetActress(id string) (*Actress, error)
	GetWish() (string, error)
	Recognition(image []byte) (string, error)
	GetFavorites(userID string) ([]*Actress, error)
	AddFavorite(*Actress) error
	RemoveFavorite(userID, actressID string) error
}

type ActressUseCase interface {
	GetActress(id string) (*Actress, error)
	GetFavorites(userID string) ([]*Actress, error)
	AddFavorite(*Actress) error
	RemoveFavorite(userID, actressID string) error
}

type ActressLineUseCase interface {
	GetWish(replyToken string) error
	GetUseInformation(replyToken string) error
	RecognitionByUser(imageID, replyToken string) error
	RecognitionByGroup(groupID, imageID, replyToken string) error
	EnableGroupRecognition(groupID, replyToken string) error
}
