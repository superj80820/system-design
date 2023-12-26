package domain

type OCRService interface {
	OCR(imageRawData string) (*OCRResponse, error)
}

type OCRResponse struct {
	OCR string `json:"ocr"`
}
