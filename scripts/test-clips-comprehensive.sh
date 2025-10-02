#!/bin/bash

# Comprehensive test of the ML clip extraction system
# Tests the full workflow from clip creation to dataset export

API_URL="${API_URL:-http://localhost:9000}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Comprehensive Clip System Test ===${NC}"
echo "API URL: $API_URL"
echo ""

# Summary variables
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Test function
run_test() {
    local test_name="$1"
    local command="$2"
    local expected="$3"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "${YELLOW}Test $TOTAL_TESTS: $test_name${NC}"

    result=$(eval "$command")
    if [[ "$result" == *"$expected"* ]] || [[ "$expected" == "CHECK_EXISTS" && -n "$result" ]]; then
        echo -e "${GREEN}✓ PASSED${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "Expected: $expected"
        echo "Got: $result"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    echo ""
}

# 1. Health check
run_test "Server health check" \
    "curl -s $API_URL/health | jq -r '.status'" \
    "ok"

# 2. List clips (should show existing clips)
CLIP_COUNT=$(curl -s $API_URL/api/v1/clips | jq '. | length')
echo -e "${BLUE}Found $CLIP_COUNT existing clips${NC}"

# 3. Get a test episode from trending
echo -e "${YELLOW}Getting test episode from trending...${NC}"
TRENDING_RESPONSE=$(curl -s -X POST $API_URL/api/v1/trending \
    -H "Content-Type: application/json" \
    -d '{"limit": 1}')

FEED_ID=$(echo "$TRENDING_RESPONSE" | jq -r '.[0].id // empty')
if [ -z "$FEED_ID" ] || [ "$FEED_ID" = "null" ]; then
    echo -e "${RED}✗ Failed to get podcast feed ID${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Got podcast feed ID: $FEED_ID${NC}"

# Sync episodes for this podcast
echo -e "${YELLOW}Syncing episodes for podcast...${NC}"
SYNC_RESPONSE=$(curl -s -X POST $API_URL/api/v1/podcasts/$FEED_ID/episodes/sync)
sleep 2  # Give sync time to complete

# Get first episode
EPISODES_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/podcasts/$FEED_ID/episodes?limit=1")
EPISODE_ID=$(echo "$EPISODES_RESPONSE" | jq -r '.[0].podcast_index_episode_id // empty')

if [ -z "$EPISODE_ID" ] || [ "$EPISODE_ID" = "null" ]; then
    echo -e "${RED}✗ Failed to get episode ID${NC}"
    echo "Episodes response: $EPISODES_RESPONSE"
    exit 1
fi
echo -e "${GREEN}✓ Got test episode ID: $EPISODE_ID${NC}\n"

# Create a new clip using the episode ID
NEW_UUID=$(curl -s -X POST $API_URL/api/v1/clips \
    -H "Content-Type: application/json" \
    -d "{\"podcast_index_episode_id\":$EPISODE_ID,\"start_time\":5,\"end_time\":20,\"label\":\"test_comprehensive\"}" \
    | jq -r '.uuid')

run_test "Create new clip" \
    "echo $NEW_UUID | grep -E '^[a-f0-9-]{36}$'" \
    "CHECK_EXISTS"

# 4. Wait for processing
echo -e "${YELLOW}Waiting for clip processing...${NC}"
sleep 3

# 5. Check clip status
run_test "Clip processing completed" \
    "curl -s $API_URL/api/v1/clips/$NEW_UUID | jq -r '.status'" \
    "ready"

# 6. Verify clip properties
CLIP_INFO=$(curl -s $API_URL/api/v1/clips/$NEW_UUID)

run_test "Clip duration is 15 seconds" \
    "echo '$CLIP_INFO' | jq -r '.duration'" \
    "15"

run_test "Clip label is correct" \
    "echo '$CLIP_INFO' | jq -r '.label'" \
    "test_comprehensive"

# 7. Test label filtering
run_test "Filter by label" \
    "curl -s '$API_URL/api/v1/clips?label=test_comprehensive' | jq '. | length'" \
    "1"

# 8. Update clip label
run_test "Update clip label" \
    "curl -s -X PUT $API_URL/api/v1/clips/$NEW_UUID/label -H 'Content-Type: application/json' -d '{\"label\":\"updated_label\"}' | jq -r '.label'" \
    "updated_label"

# 9. Test status filtering
READY_COUNT=$(curl -s "$API_URL/api/v1/clips?status=ready" | jq '. | length')
run_test "Filter by ready status" \
    "test $READY_COUNT -gt 0 && echo 'found'" \
    "found"

# 10. Test export endpoint
echo -e "${YELLOW}Testing dataset export...${NC}"
HTTP_CODE=$(curl -s -o /tmp/test_export.zip -w "%{http_code}" $API_URL/api/v1/clips/export)
run_test "Export endpoint returns 200" \
    "echo $HTTP_CODE" \
    "200"

# 11. Verify export contents
if [ -f /tmp/test_export.zip ]; then
    cd /tmp
    rm -rf test_export_dir
    mkdir test_export_dir
    cd test_export_dir
    unzip -q /tmp/test_export.zip

    run_test "Manifest file exists" \
        "test -f manifest.jsonl && echo 'exists'" \
        "exists"

    run_test "Manifest contains entries" \
        "cat manifest.jsonl | wc -l | xargs test 0 -lt && echo 'has entries'" \
        "has entries"

    # Check manifest format
    run_test "Manifest has valid JSON" \
        "cat manifest.jsonl | head -1 | jq -r '.label' > /dev/null && echo 'valid'" \
        "valid"

    cd - > /dev/null
fi

# 12. Verify audio format of clips
echo -e "${YELLOW}Verifying audio format...${NC}"
CLIP_PATH="clips/updated_label/clip_${NEW_UUID}.wav"
if [ -f "$CLIP_PATH" ]; then
    SAMPLE_RATE=$(ffprobe -v quiet -print_format json -show_streams "$CLIP_PATH" | jq -r '.streams[0].sample_rate')
    run_test "Audio sample rate is 16000 Hz" \
        "echo $SAMPLE_RATE" \
        "16000"

    CHANNELS=$(ffprobe -v quiet -print_format json -show_streams "$CLIP_PATH" | jq -r '.streams[0].channels')
    run_test "Audio is mono (1 channel)" \
        "echo $CHANNELS" \
        "1"
fi

# 13. Delete test clip
run_test "Delete clip" \
    "curl -s -o /dev/null -w '%{http_code}' -X DELETE $API_URL/api/v1/clips/$NEW_UUID" \
    "204"

# 14. Verify deletion
run_test "Clip not found after deletion" \
    "curl -s -o /dev/null -w '%{http_code}' $API_URL/api/v1/clips/$NEW_UUID" \
    "404"

# Summary
echo -e "${BLUE}=== Test Summary ===${NC}"
echo -e "Total Tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✓ All tests passed! The clip system is working correctly.${NC}"
    echo -e "The system is ready for ML training with:"
    echo -e "  • 16kHz mono WAV format"
    echo -e "  • 15-second clips (padded/cropped)"
    echo -e "  • Flexible labeling system"
    echo -e "  • JSONL manifest for training"
    exit 0
else
    echo -e "\n${RED}✗ Some tests failed. Please review the output above.${NC}"
    exit 1
fi