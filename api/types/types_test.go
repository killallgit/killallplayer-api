package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDependencies(t *testing.T) {
	deps := &Dependencies{}
	
	// Test that we can create empty dependencies
	assert.NotNil(t, deps)
	assert.Nil(t, deps.DB)
	assert.Nil(t, deps.PodcastClient)
	assert.Nil(t, deps.EpisodeService)
	assert.Nil(t, deps.EpisodeTransformer)
}

// NotFoundHandler is in api package, not types package
// So we'll just test what we have in this package