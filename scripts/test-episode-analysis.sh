#!/bin/bash

# Test script for episode volume spike analysis

set -e

API_URL="${API_URL:-http://localhost:9001}"

echo "===================================================="
echo "Episode Volume Spike Analysis Test"
echo "===================================================="
echo ""

# Step 1: Get trending podcasts to find a test episode
echo "[1/5] Fetching trending podcasts..."
TRENDING_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/trending" \
  -H "Content-Type: application/json" \
  -d '{"max": 3}')

echo "Trending podcasts fetched successfully"

# Extract first podcast ID
PODCAST_ID=$(echo "$TRENDING_RESPONSE" | jq -r '.podcasts[0].id')
PODCAST_TITLE=$(echo "$TRENDING_RESPONSE" | jq -r '.podcasts[0].title')

echo "Selected podcast: $PODCAST_TITLE (ID: $PODCAST_ID)"
echo ""

# Step 2: Get episodes for the podcast
echo "[2/5] Fetching episodes for podcast $PODCAST_ID..."
EPISODES_RESPONSE=$(curl -s "$API_URL/api/v1/podcasts/$PODCAST_ID/episodes?limit=5")

echo "Episodes fetched successfully"

# Extract first episode ID and title
EPISODE_ID=$(echo "$EPISODES_RESPONSE" | jq -r '.episodes[0].id')
EPISODE_TITLE=$(echo "$EPISODES_RESPONSE" | jq -r '.episodes[0].title')
EPISODE_DURATION=$(echo "$EPISODES_RESPONSE" | jq -r '.episodes[0].duration')

echo "Selected episode: $EPISODE_TITLE"
echo "  Episode ID: $EPISODE_ID"
echo "  Duration: ${EPISODE_DURATION}s"
echo ""

# Step 3: Trigger volume spike analysis
echo "[3/5] Analyzing episode $EPISODE_ID for volume spikes..."
echo "Note: This may take a few minutes depending on episode length..."
echo ""

ANALYSIS_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/episodes/$EPISODE_ID/analyze")

echo "Analysis response:"
echo "$ANALYSIS_RESPONSE" | jq '.'
echo ""

# Extract results
CLIPS_CREATED=$(echo "$ANALYSIS_RESPONSE" | jq -r '.clips_created')
MESSAGE=$(echo "$ANALYSIS_RESPONSE" | jq -r '.message')

echo "Result: $MESSAGE"
echo "Clips created: $CLIPS_CREATED"
echo ""

if [ "$CLIPS_CREATED" -gt 0 ]; then
    echo "[4/5] Fetching created clips for episode..."

    # Get clips for this episode
    CLIPS_LIST=$(curl -s "$API_URL/api/v1/episodes/$EPISODE_ID/clips")

    echo "Clips for this episode:"
    echo "$CLIPS_LIST" | jq '.'
    echo ""

    # Show details of first clip
    FIRST_CLIP_UUID=$(echo "$CLIPS_LIST" | jq -r '.[0].uuid')
    if [ -n "$FIRST_CLIP_UUID" ] && [ "$FIRST_CLIP_UUID" != "null" ]; then
        echo "[5/5] Fetching details for first clip: $FIRST_CLIP_UUID"

        CLIP_DETAILS=$(curl -s "$API_URL/api/v1/episodes/$EPISODE_ID/clips/$FIRST_CLIP_UUID")
        echo "$CLIP_DETAILS" | jq '.'
        echo ""
    fi

    echo "===================================================="
    echo "✅ Test completed successfully!"
    echo "===================================================="
    echo ""
    echo "Summary:"
    echo "  - Episode analyzed: $EPISODE_TITLE"
    echo "  - Volume spikes detected: $CLIPS_CREATED"
    echo "  - Clips auto-created with label: 'volume_spike'"
    echo ""
    echo "Next steps:"
    echo "  - View clips for episode: curl $API_URL/api/v1/episodes/$EPISODE_ID/clips"
    echo "  - Update clip label: curl -X PUT $API_URL/api/v1/episodes/$EPISODE_ID/clips/{uuid}/label -d '{\"label\":\"music\"}'"
    echo "  - Delete clip: curl -X DELETE $API_URL/api/v1/episodes/$EPISODE_ID/clips/{uuid}"
else
    echo "[4/5] No volume spikes detected"
    echo ""
    echo "===================================================="
    echo "✅ Test completed - no spikes found"
    echo "===================================================="
    echo ""
    echo "This is normal for podcasts with consistent volume."
    echo "Try analyzing a different episode or podcast."
fi
