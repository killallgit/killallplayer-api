package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

// ffprobeOutput represents the JSON structure returned by ffprobe
type ffprobeOutput struct {
	Format struct {
		Duration   string            `json:"duration"`
		Size       string            `json:"size"`
		Bitrate    string            `json:"bit_rate"`
		FormatName string            `json:"format_name"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
		Duration   string `json:"duration"`
	} `json:"streams"`
}

// GetMetadata extracts metadata from an audio file using ffprobe
func (f *FFmpeg) GetMetadata(ctx context.Context, filePath string) (*AudioMetadata, error) {
	args := []string{
		"-v", "quiet",
		"-show_format",
		"-show_streams",
		"-select_streams", "a:0", // Select first audio stream
		"-of", "json",
		filePath,
	}

	cmd := exec.CommandContext(ctx, f.ffprobePath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, NewProcessingError("metadata_extraction", filePath, err, stderr.String())
	}

	var output ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, NewProcessingError("metadata_parsing", filePath, err, "")
	}

	return f.parseMetadata(&output, filePath)
}

// parseMetadata converts ffprobe output to AudioMetadata
func (f *FFmpeg) parseMetadata(output *ffprobeOutput, filePath string) (*AudioMetadata, error) {
	metadata := &AudioMetadata{}

	// Parse duration
	if output.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(output.Format.Duration, 64); err == nil {
			metadata.Duration = duration
		}
	}

	// Parse file size
	if output.Format.Size != "" {
		if size, err := strconv.ParseInt(output.Format.Size, 10, 64); err == nil {
			metadata.Size = size
		}
	}

	// Parse bitrate
	if output.Format.Bitrate != "" {
		if bitrate, err := strconv.Atoi(output.Format.Bitrate); err == nil {
			metadata.Bitrate = bitrate
		}
	}

	// Parse format
	metadata.Format = output.Format.FormatName

	// Parse tags
	if tags := output.Format.Tags; tags != nil {
		metadata.Title = tags["title"]
		metadata.Artist = tags["artist"]
		metadata.Album = tags["album"]
		metadata.Year = tags["date"]
		if metadata.Year == "" {
			metadata.Year = tags["year"]
		}
	}

	// Parse audio stream information
	for _, stream := range output.Streams {
		if stream.CodecType == "audio" {
			metadata.Codec = stream.CodecName
			metadata.Channels = stream.Channels

			if stream.SampleRate != "" {
				if sampleRate, err := strconv.Atoi(stream.SampleRate); err == nil {
					metadata.SampleRate = sampleRate
				}
			}

			// Use stream duration if format duration is not available
			if metadata.Duration == 0 && stream.Duration != "" {
				if duration, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
					metadata.Duration = duration
				}
			}
			break
		}
	}

	// Validate that we have minimum required metadata
	if metadata.Duration == 0 {
		return nil, NewProcessingError("metadata_validation", filePath,
			fmt.Errorf("could not determine audio duration"), "")
	}

	return metadata, nil
}

// ValidateAudioFile checks if a file is a valid audio file that can be processed
func (f *FFmpeg) ValidateAudioFile(ctx context.Context, filePath string) error {
	metadata, err := f.GetMetadata(ctx, filePath)
	if err != nil {
		return err
	}

	// Check if duration is reasonable
	if metadata.Duration <= 0 {
		return ErrInvalidAudioFile
	}

	// Check supported formats
	supportedFormats := map[string]bool{
		"mp3":  true,
		"m4a":  true,
		"aac":  true,
		"wav":  true,
		"flac": true,
		"ogg":  true,
	}

	if !supportedFormats[metadata.Format] {
		return fmt.Errorf("unsupported audio format: %s", metadata.Format)
	}

	return nil
}
