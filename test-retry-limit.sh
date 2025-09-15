#!/bin/bash

# Test script to verify waveform generation retry limit
# This creates an episode with an invalid audio URL to trigger failures

API_URL="http://localhost:9000"
EPISODE_ID=999999  # Use a test episode ID

echo "Testing waveform generation retry limit (should fail after 3 attempts)"
echo "======================================================================="

# Test with an invalid episode ID that will cause failures
echo -e "\n1. Requesting waveform for non-existent episode $EPISODE_ID (should trigger job creation):"
curl -X GET "$API_URL/api/v1/episodes/$EPISODE_ID/waveform" 2>/dev/null | jq '.'

echo -e "\n2. Waiting 10 seconds for worker to process and fail the job..."
sleep 10

echo -e "\n3. Checking status after first failure:"
curl -X GET "$API_URL/api/v1/episodes/$EPISODE_ID/waveform" 2>/dev/null | jq '.'

echo -e "\n4. Waiting another 10 seconds for second retry..."
sleep 10

echo -e "\n5. Checking status after second failure:"
curl -X GET "$API_URL/api/v1/episodes/$EPISODE_ID/waveform" 2>/dev/null | jq '.'

echo -e "\n6. Waiting another 10 seconds for third retry..."
sleep 10

echo -e "\n7. Final check - should show permanently failed after 3 retries:"
curl -X GET "$API_URL/api/v1/episodes/$EPISODE_ID/waveform" 2>/dev/null | jq '.'

echo -e "\n\nTest complete. The job should now be permanently failed after 3 retry attempts."
echo "If you see 'Waveform generation permanently failed' in step 7, the retry limit is working correctly."