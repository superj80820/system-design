package minnano

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActressCrawlerRepo(t *testing.T) {
	actressMininanoWebCrawlerRepo := CreateActressCrawlerRepo("https://www.minnano-av.com")
	for page := 1; ; page++ {
		actresses, err := actressMininanoWebCrawlerRepo.GetActresses(page, 30)
		assert.Nil(t, err)
		t.Logf("crawler page: %d, total page: %d, items length: %d",
			actresses.CurrentPage,
			actresses.TotalPages,
			len(actresses.Items),
		)
		for _, item := range actresses.Items {
			actress, err := item.GetWithValid()
			if err != nil {
				t.Logf("get error: %+v", err)
			}
			t.Logf("actress image url: %s, image type: %s", actress.ActressPreviewURL, actress.PreviewImageType)
		}
	}
}
