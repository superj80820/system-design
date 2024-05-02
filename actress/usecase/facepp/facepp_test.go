package facepp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
)

func TestMergeKSorted(t *testing.T) {
	mergeResult := mergeKSortedByDesc([]*domain.FaceSearch{
		{
			SearchResults: []*domain.FaceSearchResults{
				{
					Confidence: 9,
				},
				{
					Confidence: 7,
				},
				{
					Confidence: 5,
				},
			},
		},
		{
			SearchResults: []*domain.FaceSearchResults{
				{
					Confidence: 90,
				},
				{
					Confidence: 36,
				},
				{
					Confidence: 2,
				},
			},
		},
		{
			SearchResults: []*domain.FaceSearchResults{
				{
					Confidence: 36,
				},
				{
					Confidence: 6,
				},
				{
					Confidence: 1,
				},
			},
		},
	})

	expected, err := json.Marshal(domain.FaceSearch{
		SearchResults: []*domain.FaceSearchResults{
			{
				Confidence: 90,
			},
			{
				Confidence: 36,
			},
			{
				Confidence: 36,
			},
			{
				Confidence: 9,
			},
			{
				Confidence: 7,
			},
			{
				Confidence: 6,
			},
			{
				Confidence: 5,
			},
			{
				Confidence: 2,
			},
			{
				Confidence: 1,
			},
		},
	})
	assert.Nil(t, err)

	result, err := json.Marshal(*mergeResult)
	assert.Nil(t, err)

	assert.Equal(t, string(expected), string(result))
}
