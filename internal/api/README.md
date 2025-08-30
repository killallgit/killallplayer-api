# API Package

This package contains all HTTP handlers and WebSocket handlers for the Podcast Player API.

## Structure
- `handlers/` - HTTP request handlers
- `websocket/` - WebSocket connection handlers
- `middleware/` - HTTP middleware functions
- `routes.go` - Route definitions and registration

## Responsibilities
- Handle HTTP requests and responses
- Manage WebSocket connections
- Validate request inputs
- Format response outputs
- Apply middleware chain

## Usage
The API package is initialized by the main server and registers all routes with the router.