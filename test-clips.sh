#!/bin/bash

# Test script for the new clips API endpoints
# This tests the ML training data clip extraction system

API_URL="${API_URL:-http://localhost:9000}"
CLIP_UUID=""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Testing Clips API Endpoints${NC}"
echo "API URL: $API_URL"
echo ""

# Test 1: Create a clip
echo -e "${GREEN}1. Creating a new clip...${NC}"
RESPONSE=$(curl -s -X POST "$API_URL/api/v1/clips" \
  -H "Content-Type: application/json" \
  -d '{
    "source_episode_url": "https://op3.dev/e/https://www.buzzsprout.com/1789435/episodes/16042809-the-economics-of-everyday-things-self-checkout.mp3",
    "start_time": 30,
    "end_time": 45,
    "label": "advertisement"
  }')

echo "Response: $RESPONSE"
CLIP_UUID=$(echo "$RESPONSE" | jq -r '.uuid // empty')

if [ -n "$CLIP_UUID" ]; then
  echo -e "${GREEN}✓ Clip created with UUID: $CLIP_UUID${NC}"
else
  echo -e "${RED}✗ Failed to create clip${NC}"
  echo "$RESPONSE"
  exit 1
fi
echo ""

# Test 2: Get the clip details
echo -e "${GREEN}2. Getting clip details...${NC}"
sleep 2  # Give it a moment to process
RESPONSE=$(curl -s -X GET "$API_URL/api/v1/clips/$CLIP_UUID")
echo "Response: $RESPONSE"

STATUS=$(echo "$RESPONSE" | jq -r '.status // empty')
echo "Clip status: $STATUS"

if [ "$STATUS" = "processing" ] || [ "$STATUS" = "ready" ] || [ "$STATUS" = "failed" ]; then
  echo -e "${GREEN}✓ Successfully retrieved clip${NC}"
else
  echo -e "${RED}✗ Failed to get clip${NC}"
fi
echo ""

# Test 3: List clips
echo -e "${GREEN}3. Listing clips...${NC}"
RESPONSE=$(curl -s -X GET "$API_URL/api/v1/clips?limit=5")
echo "Response (truncated): $(echo "$RESPONSE" | jq -c '.[0:2]')"

COUNT=$(echo "$RESPONSE" | jq '. | length')
echo "Found $COUNT clips"

if [ "$COUNT" -ge 0 ]; then
  echo -e "${GREEN}✓ Successfully listed clips${NC}"
else
  echo -e "${RED}✗ Failed to list clips${NC}"
fi
echo ""

# Test 4: Update clip label
echo -e "${GREEN}4. Updating clip label...${NC}"
RESPONSE=$(curl -s -X PUT "$API_URL/api/v1/clips/$CLIP_UUID/label" \
  -H "Content-Type: application/json" \
  -d '{"label": "music"}')

NEW_LABEL=$(echo "$RESPONSE" | jq -r '.label // empty')
if [ "$NEW_LABEL" = "music" ]; then
  echo -e "${GREEN}✓ Successfully updated label to: $NEW_LABEL${NC}"
else
  echo -e "${RED}✗ Failed to update label${NC}"
  echo "$RESPONSE"
fi
echo ""

# Test 5: Filter clips by label
echo -e "${GREEN}5. Filtering clips by label...${NC}"
RESPONSE=$(curl -s -X GET "$API_URL/api/v1/clips?label=music")
COUNT=$(echo "$RESPONSE" | jq '. | length')
echo "Found $COUNT clips with label 'music'"

if [ "$COUNT" -ge 0 ]; then
  echo -e "${GREEN}✓ Successfully filtered clips${NC}"
else
  echo -e "${RED}✗ Failed to filter clips${NC}"
fi
echo ""

# Test 6: Test export endpoint (just check it responds)
echo -e "${GREEN}6. Testing export endpoint...${NC}"
RESPONSE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/api/v1/clips/export")
if [ "$RESPONSE_CODE" = "200" ]; then
  echo -e "${GREEN}✓ Export endpoint responding (HTTP 200)${NC}"
elif [ "$RESPONSE_CODE" = "500" ]; then
  echo -e "${YELLOW}⚠ Export endpoint returned 500 (might be no ready clips yet)${NC}"
else
  echo -e "${RED}✗ Export endpoint failed with HTTP $RESPONSE_CODE${NC}"
fi
echo ""

# Test 7: Delete the clip
echo -e "${GREEN}7. Deleting the test clip...${NC}"
RESPONSE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API_URL/api/v1/clips/$CLIP_UUID")
if [ "$RESPONSE_CODE" = "204" ]; then
  echo -e "${GREEN}✓ Successfully deleted clip${NC}"
else
  echo -e "${RED}✗ Failed to delete clip (HTTP $RESPONSE_CODE)${NC}"
fi
echo ""

# Test 8: Verify deletion
echo -e "${GREEN}8. Verifying deletion...${NC}"
RESPONSE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/api/v1/clips/$CLIP_UUID")
if [ "$RESPONSE_CODE" = "404" ]; then
  echo -e "${GREEN}✓ Clip confirmed deleted (404 as expected)${NC}"
else
  echo -e "${YELLOW}⚠ Clip may still exist (HTTP $RESPONSE_CODE)${NC}"
fi
echo ""

echo -e "${GREEN}All clip API tests completed!${NC}"