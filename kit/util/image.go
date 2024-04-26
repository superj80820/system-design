package util

import "errors"

func GetImageArea(width, height float64) (float64, error) {
	if width <= 0 {
		return 0, errors.New("width less or equal zero")
	} else if height <= 0 {
		return 0, errors.New("height less or equal zero")
	}
	return width * height, nil
}
