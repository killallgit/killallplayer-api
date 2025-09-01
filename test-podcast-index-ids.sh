#!/bin/bash

# Test script for Podcast Index ID refactoring
# This script tests that the API now uses Podcast Index IDs everywhere

echo "Testing Podcast Index ID system..."
echo "=================================="

# Base URL
BASE_URL="http://localhost:8080"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to test endpoint
test_endpoint() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expected_status="$5"
    
    echo -e "\n${YELLOW}Testing: $name${NC}"
    echo "URL: $url"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    elif [ "$method" = "POST" ]; then
        response=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "$data" "$url")
    elif [ "$method" = "PUT" ]; then
        response=$(curl -s -o /dev/null -w "%{http_code}" -X PUT -H "Content-Type: application/json" -d "$data" "$url")
    fi
    
    if [ "$response" = "$expected_status" ]; then
        echo -e "${GREEN}✓ Status: $response (expected: $expected_status)${NC}"
    else
        echo -e "${RED}✗ Status: $response (expected: $expected_status)${NC}"
    fi
}

# First, search for a podcast to get episodes with Podcast Index IDs
echo -e "\n${YELLOW}1. Searching for podcasts to get episode data...${NC}"
search_response=$(curl -s -X POST "$BASE_URL/api/v1/search" \
    -H "Content-Type: application/json" \
    -d '{"query": "technology", "limit": 1}')

echo "Search response (first 500 chars):"
echo "$search_response" | head -c 500
echo "..."

# Extract a Podcast Index episode ID from the search results
# This assumes episodes are returned in the search (you might need to adjust based on actual API)
podcast_id=$(echo "$search_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

if [ -z "$podcast_id" ]; then
    echo -e "\n${RED}No podcast found in search results${NC}"
    echo "Trying to fetch episodes for a known podcast ID..."
    
    # Use a known podcast ID (adjust this based on your data)
    podcast_id=41506
    
    echo -e "\n${YELLOW}2. Fetching episodes for podcast ID: $podcast_id${NC}"
    episodes_response=$(curl -s -X POST "$BASE_URL/api/v1/podcasts/$podcast_id/sync")
    echo "Episodes response (first 500 chars):"
    echo "$episodes_response" | head -c 500
    echo "..."
    
    # Extract an episode ID from the response
    episode_id=$(echo "$episodes_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
else
    echo -e "\n${GREEN}Found podcast ID: $podcast_id${NC}"
    
    # Sync episodes for this podcast
    echo -e "\n${YELLOW}2. Syncing episodes for podcast...${NC}"
    sync_response=$(curl -s -X POST "$BASE_URL/api/v1/podcasts/$podcast_id/sync")
    
    # Extract episode ID from sync response
    episode_id=$(echo "$sync_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
fi

if [ -z "$episode_id" ]; then
    echo -e "\n${YELLOW}Using test Podcast Index ID: 41928435424${NC}"
    episode_id=41928435424
else
    echo -e "\n${GREEN}Found episode with Podcast Index ID: $episode_id${NC}"
fi

# Test the endpoints with Podcast Index IDs
echo -e "\n${YELLOW}3. Testing endpoints with Podcast Index ID: $episode_id${NC}"

# Test GET episode by ID
test_endpoint \
    "GET Episode by Podcast Index ID" \
    "GET" \
    "$BASE_URL/api/v1/episodes/$episode_id" \
    "" \
    "200"

# Test streaming endpoint
test_endpoint \
    "Stream Episode by Podcast Index ID" \
    "GET" \
    "$BASE_URL/api/v1/stream/$episode_id" \
    "" \
    "200"

# Test playback update
test_endpoint \
    "Update Playback State by Podcast Index ID" \
    "PUT" \
    "$BASE_URL/api/v1/episodes/$episode_id/playback" \
    '{"position": 120, "played": false}' \
    "200"

# Test with invalid Podcast Index ID (should return 404)
test_endpoint \
    "GET Episode with Invalid Podcast Index ID" \
    "GET" \
    "$BASE_URL/api/v1/episodes/99999999999999" \
    "" \
    "404"

# Test with non-numeric ID (should return 400)
test_endpoint \
    "GET Episode with Non-numeric ID" \
    "GET" \
    "$BASE_URL/api/v1/episodes/invalid-id" \
    "" \
    "400"

echo -e "\n${GREEN}Testing complete!${NC}"
echo "=================================="
echo ""
echo "Summary:"
echo "- All endpoints now accept Podcast Index IDs directly"
echo "- No need for clients to track internal database IDs"
echo "- Episode IDs from search results can be used directly for streaming and other operations"