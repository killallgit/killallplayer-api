/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/

// @title			Podcast Player API
// @version			1.0
// @description		A REST API for podcast discovery and episode management.
// @description		This API provides endpoints for searching podcasts, managing episodes,
// @description		and generating waveforms for audio visualization.

// @contact.name	Podcast Player API Support
// @contact.url		https://github.com/killallgit/killallplayer-api
// @contact.email	support@example.com

// @license.name	MIT
// @license.url		https://opensource.org/licenses/MIT

// @host			localhost:9000
// @BasePath		/

// @schemes			http https

// @tag.name			health
// @tag.description	Health check endpoints

// @tag.name			version
// @tag.description	API version information

// @tag.name			search
// @tag.description	Podcast search functionality

// @tag.name			episodes
// @tag.description	Episode management and playback

// @tag.name			trending
// @tag.description	Trending podcast discovery

// @tag.name			podcasts
// @tag.description	Podcast management and synchronization

// @tag.name			waveform
// @tag.description	Audio waveform generation and retrieval

// @tag.name			clips
// @tag.description	ML training audio clips extraction and management

package main

import (
	"github.com/killallgit/player-api/cmd"
	_ "github.com/killallgit/player-api/docs" // Import swagger docs
)

func main() {
	cmd.Execute()
}
