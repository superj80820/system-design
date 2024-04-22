package domain

var (
	LineOKStatus = 200
)

type LineIssueAccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

type LineVerifyToken struct {
	Iss     string   `json:"iss"`
	Sub     string   `json:"sub"`
	Aud     string   `json:"aud"`
	Exp     int      `json:"exp"`
	Iat     int      `json:"iat"`
	Nonce   string   `json:"nonce"`
	Amr     []string `json:"amr"`
	Name    string   `json:"name"`
	Picture string   `json:"picture"`
	Email   string   `json:"email"`
}

type LineUserInformation struct {
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type LineUserProfile struct {
	UserID      string `json:"user_id"`
	LineUserID  string `json:"line_user_id"`
	DisplayName string `json:"display_name"`
	PictureURL  string `json:"picture_url"`
}

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

type LineLoginAPIRepo interface {
	IssueAccessToken(code, redirectURI string) (*LineIssueAccessToken, error)
	VerifyToken(accessToken string) (*LineVerifyToken, error)
}

type LineUserRepo interface {
	Get(lineUserID string) (*LineUserProfile, error)
	Create(userID, lineUserID, DisplayName, PictureURL string) (*LineUserProfile, error)
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
