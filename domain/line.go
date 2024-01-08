package domain

var (
	LineOKStatus = 200
)

type LineRepo interface {
	Notify(message string) error
	NotifyWithToken(token, message string) error
}

type LineResponseOK struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}
