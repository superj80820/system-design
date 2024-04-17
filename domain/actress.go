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
	CreatedAt    time.Time `json:"createdat"`
	UpdatedAt    time.Time `json:"updatedat"`
}

type ActressLineRepository interface {
	GetUseInformation() (string, error)
	IsEnableGroupRecognition(ctx context.Context, groupID string) (bool, error)
	EnableGroupRecognition(ctx context.Context, groupID string) error
	DisableGroupRecognition(ctx context.Context, groupID string) error
}

type ActressRepo interface {
	AddActress(*Actress) (actressID string, err error)
	GetActress(id string) (*Actress, error)
	AddFace(actressID, faceToken, previewURL string) (faceID string, err error)
	GetActressByFaceToken(faceToken string) (*Actress, error)
	GetWish() (*Actress, error)
	GetFavorites(userID string) ([]*Actress, error)
	AddFavorite(userID, actressID string) (faceID string, err error)
	RemoveFavorite(userID, favoriteID string) error
}

type ActressUseCase interface {
	GetActress(id string) (*Actress, error)
	GetFavorites(userID string) ([]*Actress, error)
	AddFavorite(userID, actressID string) (faceID string, err error)
	RemoveFavorite(userID, actressID string) error
}

type ActressLineUseCase interface {
	GetWish(replyToken string) error
	GetUseInformation(replyToken string) error
	RecognitionByUser(ctx context.Context, imageID, replyToken string) error
	RecognitionByGroup(ctx context.Context, groupID, imageID, replyToken string) error
	EnableGroupRecognition(ctx context.Context, groupID, replyToken string) error
}
