package domain

import (
	"context"
	"time"
)

type Actress struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Preview      string    `json:"preview"`
	Detail       string    `json:"detail"`
	Romanization string    `json:"romanization"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ActressWithPagination struct {
	IsEnd     bool       `json:"is_end"`
	Total     uint       `json:"total"`
	Page      uint       `json:"page"`
	Limit     uint       `json:"limit"`
	Actresses []*Actress `json:"actresses"`
}

type FaceStatus int

const (
	FaceStatusUnknown FaceStatus = iota
	FaceStatusNotInFaceSet
	FaceStatusAlreadyInFaceSet
)

type Face struct {
	ID        int        `json:"id"`
	Token     string     `json:"token"`
	Preview   string     `json:"preview"`
	ActressID string     `json:"infoid"`
	Status    FaceStatus `json:"status"`
	CreatedAt time.Time  `json:"createdat"`
	UpdatedAt time.Time  `json:"updatedat"`
}

type ActressLineRepository interface {
	GetUseInformation() (string, error)
	IsEnableGroupRecognition(ctx context.Context, groupID string) (bool, error)
	EnableGroupRecognition(ctx context.Context, groupID string) error
	DisableGroupRecognition(ctx context.Context, groupID string) error
}

type ActressRepo interface {
	AddActress(name, preview string) (actressID string, err error)
	GetActress(id string) (*Actress, error)
	GetActresses(ctx context.Context, ids ...int32) ([]*Actress, error)
	GetActressesByPagination(ctx context.Context, page, limit int) (actresses []*Actress, size int64, isEnd bool, err error)
	AddFace(actressID, faceToken, previewURL string) (faceID string, err error)
	GetActressByFaceToken(faceToken string) (*Actress, error)
	GetFacesByStatus(status FaceStatus) ([]*Face, error)
	GetFacesByActressID(actressID string) ([]*Face, error)
	RemoveFace(faceID int) error
	SetFaceStatus(faceID int, faceSetToken string, status FaceStatus) error
	GetActressByName(name string) (*Actress, error)
	SetActressPreview(actressID, previewURL string) error
	GetWish() (*Actress, error)
	GetFavoritesPagination(ctx context.Context, userID string, page, limit uint) (*ActressWithPagination, error)
	AddFavorite(userID, actressID string) error
	RemoveFavorite(userID, actressID string) error
}

type ActressReverseIndexUseCase interface {
	AddData(actressName string, actressID string)
	Search(actressName string) ([]int32, error)
	SearchWithPagination(actressName string, page, limit uint) (allData []int32, total uint, isEnd bool, err error)
}

type ActressUseCase interface {
	GetActress(id string) (*Actress, error)
	GetFavoritesPagination(ctx context.Context, userID string, page, limit uint) (*ActressWithPagination, error)
	AddFavorite(userID, actressID string) (err error)
	RemoveFavorite(userID, actressID string) error
	SearchActressByFace(faceImage []byte) ([]*Actress, error)
	SearchActressByNamePagination(ctx context.Context, name string, page, limit uint) (*ActressWithPagination, error)
}

type ActressLineUseCase interface {
	GetWish(replyToken string) error
	GetUseInformation(replyToken string) error
	RecognitionByUser(ctx context.Context, imageID, replyToken string) error
	RecognitionByGroup(ctx context.Context, groupID, imageID, replyToken string) error
	EnableGroupRecognition(ctx context.Context, groupID, replyToken string) error
}

type ActressCrawlerData struct {
	ActressName       string
	ActressPreviewURL string
	PreviewImageType  ImageType
}

type ActressCrawlerDataPagination struct {
	Count       int
	CurrentPage int
	TotalPages  int
	Items       []ActressCrawlerProvider
}

type ActressCrawlerProvider interface {
	GetWithValid() (*ActressCrawlerData, error)
	GetImage() (ImageGetter, error)
}

type ActressCrawlerRepo interface {
	GetActresses(page, limit int) (*ActressCrawlerDataPagination, error)
}

type ActressCrawlerUseCase interface {
	Process(ctx context.Context)
	Done() <-chan struct{}
	Err() error
}
