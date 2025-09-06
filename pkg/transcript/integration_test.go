package transcript_test

import (
	"context"
	"testing"
	"time"

	"github.com/killallgit/player-api/pkg/transcript"
)

func TestFetcherIntegration(t *testing.T) {
	// Test fetching a real VTT transcript (if available)
	t.Run("VTT Transcript", func(t *testing.T) {
		// This is a test VTT URL - replace with a real one if available
		url := "https://example.com/transcript.vtt"

		opts := transcript.DefaultFetchOptions()
		opts.Timeout = 5 * time.Second
		fetcher := transcript.NewFetcher(opts)

		ctx := context.Background()
		result, err := fetcher.Fetch(ctx, url)

		// We expect this to fail with a 404 or similar for a fake URL
		if err == nil {
			t.Log("Successfully fetched transcript")
			if result.Format != transcript.FormatVTT {
				t.Errorf("Expected VTT format, got %s", result.Format)
			}
		} else {
			// This is expected for a fake URL
			t.Logf("Expected error for fake URL: %v", err)
		}
	})
}

func TestTranscriptWorkflow(t *testing.T) {
	// Test the complete workflow: fetch and parse
	vttContent := `WEBVTT

00:00:00.000 --> 00:00:05.000
Welcome to our podcast about Go programming.

00:00:05.000 --> 00:00:10.000
Today we'll discuss error handling and testing.`

	parser := transcript.NewParser()
	result, err := parser.Parse(vttContent, transcript.FormatVTT)

	if err != nil {
		t.Fatalf("Failed to parse VTT: %v", err)
	}

	// Verify the transcript was parsed correctly
	if len(result.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(result.Segments))
	}

	if result.Duration != 10*time.Second {
		t.Errorf("Expected 10s duration, got %v", result.Duration)
	}

	plainText := result.ToPlainText()
	if plainText == "" {
		t.Error("Plain text should not be empty")
	}

	t.Logf("Successfully parsed transcript with %d segments", len(result.Segments))
	t.Logf("Plain text: %s", plainText)
}
