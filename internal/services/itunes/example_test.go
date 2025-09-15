package itunes_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/killallgit/player-api/internal/services/itunes"
)

func ExampleClient_LookupPodcast() {
	// Create client with conservative rate limits
	client := itunes.NewClient(itunes.Config{
		RequestsPerMinute: 20, // Use documented rate limit
		BurstSize:         2,
		Timeout:           10 * time.Second,
	})

	ctx := context.Background()

	// Look up "The Backup Wrap-Up" podcast
	podcast, err := client.LookupPodcast(ctx, 1469663053)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Podcast: %s by %s\n", podcast.Title, podcast.Author)
	fmt.Printf("Feed URL: %s\n", podcast.FeedURL)
	fmt.Printf("Episodes: %d\n", podcast.EpisodeCount)
	// Output:
	// Podcast: The Backup Wrap-Up by W. Curtis Preston (Mr. Backup)
	// Feed URL: https://feeds.captivate.fm/backupwrapup/
	// Episodes: 317
}

func ExampleCachedClient_LookupPodcastWithEpisodes() {
	// Create cached client for better performance
	client := itunes.NewCachedClient(
		itunes.Config{
			RequestsPerMinute: 20,
			BurstSize:         2,
			Timeout:           10 * time.Second,
		},
		nil, // Use default in-memory cache
		15*time.Minute,
	)

	ctx := context.Background()

	// Look up podcast with latest 5 episodes
	result, err := client.LookupPodcastWithEpisodes(ctx, 1469663053, 5)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Podcast: %s\n", result.Podcast.Title)
	fmt.Printf("Latest %d episodes:\n", len(result.Episodes))
	for i, episode := range result.Episodes {
		fmt.Printf("%d. %s (%d ms)\n", i+1, episode.Title, episode.Duration)
	}
}
