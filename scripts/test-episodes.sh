#!/bin/bash

# Test script for episode management endpoints

BASE_URL="http://localhost:8080"
API_BASE="$BASE_URL/api/v1"

echo "=== Testing Episode Management API ==="
echo ""

# Test 1: Get episodes for a podcast
echo "1. Getting episodes for podcast ID 42"
curl -X GET "$API_BASE/podcasts/42/episodes?page=1&limit=10" \
  -H "Content-Type: application/json" | jq '.'
echo ""

# Test 2: Sync episodes from Podcast Index
echo "2. Syncing episodes from Podcast Index for podcast ID 42"
curl -X POST "$API_BASE/podcasts/42/episodes/sync?limit=5" \
  -H "Content-Type: application/json" | jq '.'
echo ""

# Test 3: Get specific episode
echo "3. Getting episode with ID 1"
curl -X GET "$API_BASE/episodes/1" \
  -H "Content-Type: application/json" | jq '.'
echo ""

# Test 4: Update playback state
echo "4. Updating playback state for episode 1"
curl -X PUT "$API_BASE/episodes/1/playback" \
  -H "Content-Type: application/json" \
  -d '{
    "position": 1234,
    "played": false
  }' | jq '.'
echo ""

# Test 5: Get episode metadata
echo "5. Getting metadata for episode 1"
curl -X GET "$API_BASE/episodes/1/metadata" \
  -H "Content-Type: application/json" | jq '.'
echo ""

# Test 6: Search episodes
echo "6. Searching for episodes"
curl -X POST "$API_BASE/episodes/search" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "technology",
    "limit": 5
  }' | jq '.'
echo ""

echo "=== Episode Management API Tests Complete ==="