/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import "github.com/killallgit/player-api/cmd"

// @title           Podcast Player API
// @version         1.0.0
// @description     A podcast discovery and streaming API with episode management
// @termsOfService  http://swagger.io/terms/
// @contact.name    API Support
// @contact.url     https://github.com/killallgit/killallplayer-api
// @contact.email   support@example.com
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
// @host            localhost:8080
// @BasePath        /
// @schemes         http https
// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        Authorization
// @description                 Static API token for Swagger UI access
func main() {
	cmd.Execute()
}
