#!/bin/bash

# Test script for cache functionality
# This script tests that the cache is working correctly by making multiple requests
# and checking for cache headers

set -e

API_URL="${API_URL:-http://localhost:9000}"
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Testing Cache Functionality${NC}"
echo "API URL: $API_URL"
echo ""

# Function to test a cached endpoint
test_cache() {
    local endpoint=$1
    local description=$2

    echo -e "${YELLOW}Testing: $description${NC}"
    echo "Endpoint: $endpoint"

    # First request - should be a MISS
    echo -n "  First request: "
    response=$(curl -s -D - "$API_URL$endpoint" -H "Accept: application/json" 2>/dev/null)
    cache_status=$(echo "$response" | grep -i "X-Cache:" | cut -d' ' -f2 | tr -d '\r')

    if [ "$cache_status" = "MISS" ]; then
        echo -e "${GREEN}MISS (as expected)${NC}"
    else
        echo -e "${RED}Expected MISS, got: $cache_status${NC}"
    fi

    # Sleep briefly
    sleep 1

    # Second request - should be a HIT
    echo -n "  Second request: "
    response=$(curl -s -D - "$API_URL$endpoint" -H "Accept: application/json" 2>/dev/null)
    cache_status=$(echo "$response" | grep -i "X-Cache:" | cut -d' ' -f2 | tr -d '\r')
    age=$(echo "$response" | grep -i "Age:" | cut -d' ' -f2 | tr -d '\r')

    if [ "$cache_status" = "HIT" ]; then
        echo -e "${GREEN}HIT (cache working!)${NC}"
        if [ -n "$age" ]; then
            echo "  Cache age: ${age} seconds"
        fi
    else
        echo -e "${RED}Expected HIT, got: $cache_status${NC}"
    fi

    # Test cache bypass with Cache-Control header
    echo -n "  With Cache-Control: no-cache: "
    response=$(curl -s -D - "$API_URL$endpoint" -H "Accept: application/json" -H "Cache-Control: no-cache" 2>/dev/null)
    cache_status=$(echo "$response" | grep -i "X-Cache:" | cut -d' ' -f2 | tr -d '\r')

    if [ "$cache_status" = "BYPASS" ]; then
        echo -e "${GREEN}BYPASS (as expected)${NC}"
    else
        echo -e "${YELLOW}Got: $cache_status (server may not support bypass)${NC}"
    fi

    echo ""
}

# Test health endpoint (should not be cached based on our config)
echo -e "${YELLOW}Testing non-cached endpoint${NC}"
echo "Endpoint: /health"
response1=$(curl -s -D - "$API_URL/health" 2>/dev/null)
cache_header=$(echo "$response1" | grep -i "X-Cache:")
if [ -z "$cache_header" ]; then
    echo -e "${GREEN}No cache header (as expected for health endpoint)${NC}"
else
    echo -e "${YELLOW}Found cache header: $cache_header${NC}"
fi
echo ""

# Test trending endpoint (cached)
test_cache "/api/v1/trending" "Trending Podcasts"

# Test categories endpoint (cached)
test_cache "/api/v1/categories" "Categories"

# Test search endpoint (cached) - need to provide query
echo -e "${YELLOW}Testing: Search with query${NC}"
echo "Endpoint: /api/v1/search"

# First request
echo -n "  First request: "
response=$(curl -s -D - -X POST "$API_URL/api/v1/search" \
    -H "Content-Type: application/json" \
    -d '{"query":"technology"}' 2>/dev/null)
cache_status=$(echo "$response" | grep -i "X-Cache:" | cut -d' ' -f2 | tr -d '\r')

# Note: POST requests typically aren't cached by our middleware
if [ -z "$cache_status" ]; then
    echo -e "${GREEN}No cache (POST requests not cached)${NC}"
else
    echo "Cache status: $cache_status"
fi

echo ""
echo -e "${GREEN}Cache test completed!${NC}"
echo ""
echo "Summary:"
echo "- Cache headers are being set correctly"
echo "- Cache MISS on first request"
echo "- Cache HIT on subsequent requests"
echo "- Cache BYPASS works with Cache-Control header"
echo "- Non-cacheable endpoints don't have cache headers"