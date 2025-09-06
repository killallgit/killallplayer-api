package transcript

import (
	"strings"
	"testing"
	"time"
)

func TestParseVTT(t *testing.T) {
	vttContent := `WEBVTT

00:00:00.000 --> 00:00:03.000
Welcome to the podcast.

00:00:03.000 --> 00:00:06.000
Today we're discussing Go programming.

00:00:06.000 --> 00:00:10.000
Let's dive into the basics.`

	parser := NewParser()
	transcript, err := parser.Parse(vttContent, FormatVTT)

	if err != nil {
		t.Fatalf("Failed to parse VTT: %v", err)
	}

	if len(transcript.Segments) != 3 {
		t.Errorf("Expected 3 segments, got %d", len(transcript.Segments))
	}

	if transcript.Segments[0].Text != "Welcome to the podcast." {
		t.Errorf("First segment text mismatch: %s", transcript.Segments[0].Text)
	}

	if transcript.Duration != 10*time.Second {
		t.Errorf("Expected duration of 10s, got %v", transcript.Duration)
	}

	fullText := transcript.ToPlainText()
	if !strings.Contains(fullText, "Welcome to the podcast") {
		t.Errorf("Full text doesn't contain expected content: %s", fullText)
	}
}

func TestParseSRT(t *testing.T) {
	srtContent := `1
00:00:00,000 --> 00:00:03,000
Welcome to the podcast.

2
00:00:03,000 --> 00:00:06,000
Today we're discussing Go programming.

3
00:00:06,000 --> 00:00:10,000
Let's dive into the basics.`

	parser := NewParser()
	transcript, err := parser.Parse(srtContent, FormatSRT)

	if err != nil {
		t.Fatalf("Failed to parse SRT: %v", err)
	}

	if len(transcript.Segments) != 3 {
		t.Errorf("Expected 3 segments, got %d", len(transcript.Segments))
	}

	if transcript.Segments[1].Text != "Today we're discussing Go programming." {
		t.Errorf("Second segment text mismatch: %s", transcript.Segments[1].Text)
	}

	if transcript.Duration != 10*time.Second {
		t.Errorf("Expected duration of 10s, got %v", transcript.Duration)
	}
}

func TestParseJSON(t *testing.T) {
	jsonContent := `[
		{
			"startTime": 0,
			"endTime": 3,
			"text": "Welcome to the podcast."
		},
		{
			"startTime": 3,
			"endTime": 6,
			"text": "Today we're discussing Go programming."
		},
		{
			"startTime": 6,
			"endTime": 10,
			"text": "Let's dive into the basics."
		}
	]`

	parser := NewParser()
	transcript, err := parser.Parse(jsonContent, FormatJSON)

	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(transcript.Segments) != 3 {
		t.Errorf("Expected 3 segments, got %d", len(transcript.Segments))
	}

	if transcript.Segments[2].Text != "Let's dive into the basics." {
		t.Errorf("Third segment text mismatch: %s", transcript.Segments[2].Text)
	}

	if transcript.Duration != 10*time.Second {
		t.Errorf("Expected duration of 10s, got %v", transcript.Duration)
	}
}

func TestParsePlainText(t *testing.T) {
	textContent := `Welcome to the podcast. Today we're discussing Go programming. Let's dive into the basics.`

	parser := NewParser()
	transcript, err := parser.Parse(textContent, FormatText)

	if err != nil {
		t.Fatalf("Failed to parse plain text: %v", err)
	}

	if len(transcript.Segments) != 0 {
		t.Errorf("Expected 0 segments for plain text, got %d", len(transcript.Segments))
	}

	if transcript.FullText != strings.TrimSpace(textContent) {
		t.Errorf("Full text mismatch: %s", transcript.FullText)
	}

	if transcript.Duration != 0 {
		t.Errorf("Expected duration of 0 for plain text, got %v", transcript.Duration)
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		url         string
		contentType string
		content     string
		expected    TranscriptFormat
	}{
		{"https://example.com/transcript.vtt", "", "", FormatVTT},
		{"https://example.com/transcript.srt", "", "", FormatSRT},
		{"https://example.com/transcript.json", "", "", FormatJSON},
		{"https://example.com/transcript.txt", "", "", FormatText},
		{"https://example.com/transcript", "text/vtt", "", FormatVTT},
		{"https://example.com/transcript", "application/x-subrip", "", FormatSRT},
		{"https://example.com/transcript", "application/json", "", FormatJSON},
		{"https://example.com/transcript", "", "WEBVTT\n\n00:00:00.000 --> 00:00:03.000", FormatVTT},
		{"https://example.com/transcript", "", "1\n00:00:00,000 --> 00:00:03,000", FormatSRT},
		{"https://example.com/transcript", "", "[{\"text\":\"test\"}]", FormatJSON},
		{"https://example.com/transcript", "", "Just plain text", FormatText},
	}

	for _, tt := range tests {
		format := detectFormat(tt.url, tt.contentType, tt.content)
		if format != tt.expected {
			t.Errorf("detectFormat(%s, %s, %s...) = %s, want %s",
				tt.url, tt.contentType, tt.content[:min(20, len(tt.content))], format, tt.expected)
		}
	}
}
