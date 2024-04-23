package domain

type ImageType string

const (
	ImageTypeUnknown ImageType = "unknown"
	ImageTypeJPG     ImageType = "jpg"
	ImageTypePNG     ImageType = "png"
	ImageTypeGIF     ImageType = "gif"
)
