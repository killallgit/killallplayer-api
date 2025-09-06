#!/bin/bash

echo "=== Testing Complete Podcast → Episode Flow ==="
echo

echo "1. Getting trending podcasts..."
TRENDING_RESPONSE=$(curl -s "http://localhost:8080/api/v1/trending?limit=1")
echo "$TRENDING_RESPONSE" | jq .

# Extract podcast ID from trending response
PODCAST_ID=$(echo "$TRENDING_RESPONSE" | jq -r '.feeds[0].id')
echo
echo "Found podcast ID: $PODCAST_ID"
echo

echo "2. Getting episodes for podcast $PODCAST_ID..."
EPISODES_RESPONSE=$(curl -s "http://localhost:8080/api/v1/podcasts/$PODCAST_ID/episodes?limit=3")
echo "$EPISODES_RESPONSE" | jq '.items[0:2]'  # Show first 2 episodes

# Extract first episode ID
EPISODE_ID=$(echo "$EPISODES_RESPONSE" | jq -r '.items[0].id')
echo
echo "Found episode ID: $EPISODE_ID"
echo

echo "3. Getting details for episode $EPISODE_ID..."
curl -s "http://localhost:8080/api/v1/episodes/$EPISODE_ID" | jq .

echo
echo "=== Flow Complete! ==="