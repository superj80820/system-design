package domain

var (
	LineOKStatus = 200
)

type LineAPIRepo interface {
	Notify(message string) error
	NotifyWithToken(token, message string) error
	ReplyText(token, text string) error
	GetImage(imageID string) ([]byte, error)
	VerifyLIFF(liffID, accessToken string) error
}

type LineResponseOK struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type LineLIFFUseCase interface {
	VerifyLIFF(liffID, accessToken string) error
}
