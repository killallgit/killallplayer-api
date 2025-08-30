# FFmpeg Package

This package provides a Go wrapper around FFmpeg and FFprobe for audio processing.

## Features
- Audio metadata extraction
- Audio format conversion
- Stream processing
- Duration detection
- Codec information extraction
- Audio chunking for transcription

## Structure
- `ffmpeg.go` - FFmpeg command wrapper
- `ffprobe.go` - FFprobe metadata extraction
- `types.go` - Data structures for audio metadata
- `errors.go` - Custom error types

## Dependencies
- Requires FFmpeg and FFprobe binaries installed on the system
- Binary paths configured in application config

## Usage
```go
ffmpeg := ffmpeg.New(ffmpegPath, ffprobePath)
metadata, err := ffmpeg.GetMetadata("audio.mp3")
```