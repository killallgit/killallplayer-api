package episodes

import "fmt"

// DefaultKeyGenerator implements CacheKeyGenerator with a consistent key format
type DefaultKeyGenerator struct {
	prefix string
}

// NewKeyGenerator creates a new key generator with an optional prefix
func NewKeyGenerator(prefix string) CacheKeyGenerator {
	if prefix == "" {
		prefix = "episode"
	}
	return &DefaultKeyGenerator{prefix: prefix}
}

// EpisodeByID generates a cache key for an episode by ID
func (g *DefaultKeyGenerator) EpisodeByID(id uint) string {
	return fmt.Sprintf("%s:id:%d", g.prefix, id)
}

// EpisodeByGUID generates a cache key for an episode by GUID
func (g *DefaultKeyGenerator) EpisodeByGUID(guid string) string {
	return fmt.Sprintf("%s:guid:%s", g.prefix, guid)
}

// EpisodeByPodcastIndexID generates a cache key for an episode by Podcast Index ID
func (g *DefaultKeyGenerator) EpisodeByPodcastIndexID(podcastIndexID int64) string {
	return fmt.Sprintf("%s:podcastindex:%d", g.prefix, podcastIndexID)
}

// EpisodesByPodcast generates a cache key for episodes by podcast
func (g *DefaultKeyGenerator) EpisodesByPodcast(podcastID uint, page, limit int) string {
	return fmt.Sprintf("%s:podcast:%d:page:%d:limit:%d", g.prefix, podcastID, page, limit)
}

// RecentEpisodes generates a cache key for recent episodes
func (g *DefaultKeyGenerator) RecentEpisodes(limit int) string {
	return fmt.Sprintf("%s:recent:limit:%d", g.prefix, limit)
}

// PodcastPattern generates a pattern for invalidating all episodes for a podcast
func (g *DefaultKeyGenerator) PodcastPattern(podcastID uint) string {
	return fmt.Sprintf("%s:podcast:%d:*", g.prefix, podcastID)
}
