package domain

var (
	LineOKStatus = 200
)

type LineNotifyAPIRepo interface {
	Notify(message string) error
	NotifyWithToken(token, message string) error
}

type LineMessageAPIRepo interface {
	Reply(token string, messages ...string) error
	GetImage(imageID string) ([]byte, error)
}

type LineLIFFAPIRepo interface {
	VerifyLIFF(liffID, accessToken string) error
}

type LineTemplateRepo interface {
	ApplyTemplate(name string, args ...any) (string, error)
	ApplyText(text string) string
}

type LineResponseOK struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type LineLIFFUseCase interface {
	VerifyLIFF(liffID, accessToken string) error
}
