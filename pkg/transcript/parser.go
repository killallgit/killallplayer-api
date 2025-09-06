package transcript

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TranscriptFormat represents the format of a transcript
type TranscriptFormat string

const (
	FormatVTT  TranscriptFormat = "vtt"
	FormatSRT  TranscriptFormat = "srt"
	FormatJSON TranscriptFormat = "json"
	FormatText TranscriptFormat = "text"
)

// Segment represents a transcript segment with timing information
type Segment struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// Transcript represents a parsed transcript
type Transcript struct {
	Format   TranscriptFormat
	Segments []Segment
	FullText string
	Duration time.Duration
}

// Parser handles parsing different transcript formats
type Parser struct{}

// NewParser creates a new transcript parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses transcript content based on its format
func (p *Parser) Parse(content string, format TranscriptFormat) (*Transcript, error) {
	switch format {
	case FormatVTT:
		return p.parseVTT(content)
	case FormatSRT:
		return p.parseSRT(content)
	case FormatJSON:
		return p.parseJSON(content)
	case FormatText:
		return p.parseText(content)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// parseVTT parses WebVTT format transcripts
func (p *Parser) parseVTT(content string) (*Transcript, error) {
	transcript := &Transcript{
		Format:   FormatVTT,
		Segments: []Segment{},
	}

	lines := strings.Split(content, "\n")
	var currentSegment *Segment
	var textBuilder strings.Builder
	var fullTextBuilder strings.Builder

	// Regular expression for VTT timestamp line (e.g., "00:00:01.000 --> 00:00:05.000")
	timestampRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}\.\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}\.\d{3})`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip WEBVTT header and empty lines
		if strings.HasPrefix(line, "WEBVTT") || strings.HasPrefix(line, "NOTE") || line == "" {
			continue
		}

		// Check for timestamp line
		if matches := timestampRegex.FindStringSubmatch(line); matches != nil {
			// Save previous segment if exists
			if currentSegment != nil && textBuilder.Len() > 0 {
				currentSegment.Text = strings.TrimSpace(textBuilder.String())
				transcript.Segments = append(transcript.Segments, *currentSegment)
				fullTextBuilder.WriteString(currentSegment.Text)
				fullTextBuilder.WriteString(" ")
				textBuilder.Reset()
			}

			// Parse timestamps
			start, _ := parseVTTTimestamp(matches[1])
			end, _ := parseVTTTimestamp(matches[2])

			currentSegment = &Segment{
				Start: start,
				End:   end,
			}
		} else if currentSegment != nil && !strings.Contains(line, "-->") {
			// This is subtitle text
			if textBuilder.Len() > 0 {
				textBuilder.WriteString(" ")
			}
			// Remove VTT tags like <v Speaker>
			cleanLine := removeVTTTags(line)
			textBuilder.WriteString(cleanLine)
		}
	}

	// Don't forget the last segment
	if currentSegment != nil && textBuilder.Len() > 0 {
		currentSegment.Text = strings.TrimSpace(textBuilder.String())
		transcript.Segments = append(transcript.Segments, *currentSegment)
		fullTextBuilder.WriteString(currentSegment.Text)
	}

	transcript.FullText = strings.TrimSpace(fullTextBuilder.String())

	// Calculate total duration from last segment
	if len(transcript.Segments) > 0 {
		transcript.Duration = transcript.Segments[len(transcript.Segments)-1].End
	}

	return transcript, nil
}

// parseSRT parses SRT format transcripts
func (p *Parser) parseSRT(content string) (*Transcript, error) {
	transcript := &Transcript{
		Format:   FormatSRT,
		Segments: []Segment{},
	}

	lines := strings.Split(content, "\n")
	var currentSegment *Segment
	var textBuilder strings.Builder
	var fullTextBuilder strings.Builder

	// Regular expression for SRT timestamp line (e.g., "00:00:01,000 --> 00:00:05,000")
	timestampRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2},\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2},\d{3})`)
	inText := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip sequence numbers and empty lines
		if line == "" {
			if currentSegment != nil && textBuilder.Len() > 0 {
				currentSegment.Text = strings.TrimSpace(textBuilder.String())
				transcript.Segments = append(transcript.Segments, *currentSegment)
				fullTextBuilder.WriteString(currentSegment.Text)
				fullTextBuilder.WriteString(" ")
				textBuilder.Reset()
				currentSegment = nil
				inText = false
			}
			continue
		}

		// Skip sequence numbers (lines with only digits)
		if regexp.MustCompile(`^\d+$`).MatchString(line) {
			continue
		}

		// Check for timestamp line
		if matches := timestampRegex.FindStringSubmatch(line); matches != nil {
			start, _ := parseSRTTimestamp(matches[1])
			end, _ := parseSRTTimestamp(matches[2])

			currentSegment = &Segment{
				Start: start,
				End:   end,
			}
			inText = true
		} else if inText && currentSegment != nil {
			// This is subtitle text
			if textBuilder.Len() > 0 {
				textBuilder.WriteString(" ")
			}
			textBuilder.WriteString(line)
		}
	}

	// Don't forget the last segment
	if currentSegment != nil && textBuilder.Len() > 0 {
		currentSegment.Text = strings.TrimSpace(textBuilder.String())
		transcript.Segments = append(transcript.Segments, *currentSegment)
		fullTextBuilder.WriteString(currentSegment.Text)
	}

	transcript.FullText = strings.TrimSpace(fullTextBuilder.String())

	// Calculate total duration from last segment
	if len(transcript.Segments) > 0 {
		transcript.Duration = transcript.Segments[len(transcript.Segments)-1].End
	}

	return transcript, nil
}

// parseJSON parses JSON format transcripts (common podcast transcript format)
func (p *Parser) parseJSON(content string) (*Transcript, error) {
	transcript := &Transcript{
		Format:   FormatJSON,
		Segments: []Segment{},
	}

	// Try to parse as array of segments
	var segments []struct {
		Start     float64 `json:"startTime"`
		StartTime float64 `json:"start_time"`
		End       float64 `json:"endTime"`
		EndTime   float64 `json:"end_time"`
		Text      string  `json:"text"`
		Body      string  `json:"body"`
	}

	if err := json.Unmarshal([]byte(content), &segments); err != nil {
		// Try as object with segments array
		var obj struct {
			Segments []struct {
				Start     float64 `json:"startTime"`
				StartTime float64 `json:"start_time"`
				End       float64 `json:"endTime"`
				EndTime   float64 `json:"end_time"`
				Text      string  `json:"text"`
				Body      string  `json:"body"`
			} `json:"segments"`
		}

		if err := json.Unmarshal([]byte(content), &obj); err != nil {
			return nil, fmt.Errorf("failed to parse JSON transcript: %w", err)
		}
		segments = obj.Segments
	}

	var fullTextBuilder strings.Builder

	for _, seg := range segments {
		// Handle different field names
		start := seg.Start
		if start == 0 && seg.StartTime > 0 {
			start = seg.StartTime
		}

		end := seg.End
		if end == 0 && seg.EndTime > 0 {
			end = seg.EndTime
		}

		text := seg.Text
		if text == "" && seg.Body != "" {
			text = seg.Body
		}

		segment := Segment{
			Start: time.Duration(start * float64(time.Second)),
			End:   time.Duration(end * float64(time.Second)),
			Text:  strings.TrimSpace(text),
		}

		transcript.Segments = append(transcript.Segments, segment)
		fullTextBuilder.WriteString(segment.Text)
		fullTextBuilder.WriteString(" ")
	}

	transcript.FullText = strings.TrimSpace(fullTextBuilder.String())

	// Calculate total duration from last segment
	if len(transcript.Segments) > 0 {
		transcript.Duration = transcript.Segments[len(transcript.Segments)-1].End
	}

	return transcript, nil
}

// parseText parses plain text transcripts (no timing information)
func (p *Parser) parseText(content string) (*Transcript, error) {
	// For plain text, we just return the full text without segments
	transcript := &Transcript{
		Format:   FormatText,
		Segments: []Segment{},
		FullText: strings.TrimSpace(content),
		Duration: 0, // Unknown duration for plain text
	}

	return transcript, nil
}

// parseVTTTimestamp parses a VTT timestamp (HH:MM:SS.mmm)
func parseVTTTimestamp(timestamp string) (time.Duration, error) {
	parts := strings.Split(timestamp, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid VTT timestamp: %s", timestamp)
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])

	secParts := strings.Split(parts[2], ".")
	seconds, _ := strconv.Atoi(secParts[0])
	milliseconds := 0
	if len(secParts) > 1 {
		milliseconds, _ = strconv.Atoi(secParts[1])
	}

	duration := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(milliseconds)*time.Millisecond

	return duration, nil
}

// parseSRTTimestamp parses an SRT timestamp (HH:MM:SS,mmm)
func parseSRTTimestamp(timestamp string) (time.Duration, error) {
	// Replace comma with dot for easier parsing
	timestamp = strings.Replace(timestamp, ",", ".", 1)
	return parseVTTTimestamp(timestamp)
}

// removeVTTTags removes VTT-specific tags from text
func removeVTTTags(text string) string {
	// Remove voice tags <v Speaker>
	voiceRegex := regexp.MustCompile(`<v[^>]*>`)
	text = voiceRegex.ReplaceAllString(text, "")

	// Remove other common tags
	text = strings.ReplaceAll(text, "</v>", "")
	text = strings.ReplaceAll(text, "<i>", "")
	text = strings.ReplaceAll(text, "</i>", "")
	text = strings.ReplaceAll(text, "<b>", "")
	text = strings.ReplaceAll(text, "</b>", "")
	text = strings.ReplaceAll(text, "<u>", "")
	text = strings.ReplaceAll(text, "</u>", "")

	return strings.TrimSpace(text)
}

// ToPlainText converts a transcript to plain text format
func (t *Transcript) ToPlainText() string {
	if t.FullText != "" {
		return t.FullText
	}

	var builder strings.Builder
	for _, segment := range t.Segments {
		builder.WriteString(segment.Text)
		builder.WriteString(" ")
	}

	return strings.TrimSpace(builder.String())
}
