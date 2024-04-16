package repository

import (
	"github.com/superj80820/system-design/domain"
)

type lineLIFFAPIRepo struct {
	url string
}

func CreateLineLIFFRepo(url, token string) domain.LineLIFFAPIRepo {
	return &lineLIFFAPIRepo{
		url: url,
	}
}

// VerifyLIFF implements domain.LineAPIRepo.
func (l *lineLIFFAPIRepo) VerifyLIFF(liffID, accessToken string) error {
	panic("unimplemented")
}
