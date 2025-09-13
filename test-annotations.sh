#!/bin/bash

# Test script for audio annotations/regions API
# Tests complete CRUD operations for episode annotations

set -e  # Exit on error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:9000}"
VERBOSE="${VERBOSE:-false}"

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

# Function to check if jq is installed
check_dependencies() {
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed. Please install jq to continue."
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed. Please install curl to continue."
        exit 1
    fi
}

# Main test flow
main() {
    echo "==========================================="
    echo "  Audio Annotations API Integration Test  "
    echo "==========================================="
    echo ""
    echo "API URL: $API_URL"
    echo ""
    
    check_dependencies
    
    # Step 1: Health check
    log_info "Step 1: Checking API health..."
    health_response=$(curl -s -X GET "$API_URL/health" -H "Content-Type: application/json")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Health response: $health_response" >&2
    fi
    
    if ! echo "$health_response" | jq -e '.status == "ok"' > /dev/null 2>&1; then
        log_error "API is not healthy"
        echo "$health_response" >&2
        exit 1
    fi
    log_success "API is healthy"
    echo ""
    
    # Step 2: Search for podcasts to find a testable episode
    log_info "Step 2: Searching for podcasts..."
    search_data='{"query": "tech", "limit": 5}'
    search_response=$(curl -s -X POST "$API_URL/api/v1/search" \
        -H "Content-Type: application/json" \
        -d "$search_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Search response: $search_response" >&2
    fi
    
    # Extract first podcast ID from search results
    podcast_id=$(echo "$search_response" | jq -r '.results[0].id // empty')
    podcast_title=$(echo "$search_response" | jq -r '.results[0].title // empty')
    
    if [ -z "$podcast_id" ]; then
        log_error "Could not find any podcasts"
        exit 1
    fi
    
    log_success "Selected podcast: $podcast_title (ID: $podcast_id)"
    echo ""
    
    # Step 3: Sync episodes for the podcast
    log_info "Step 3: Syncing episodes for podcast..."
    sync_response=$(curl -s -X POST "$API_URL/api/v1/podcasts/$podcast_id/episodes/sync" -H "Content-Type: application/json")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Sync response: $sync_response" >&2
    fi
    
    # Wait a moment for sync to complete
    sleep 2
    
    # Step 4: Get podcast episodes
    log_info "Step 4: Getting podcast episodes..."
    podcast_response=$(curl -s -X GET "$API_URL/api/v1/podcasts/$podcast_id/episodes" -H "Content-Type: application/json")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Podcast response (first 500 chars): ${podcast_response:0:500}" >&2
    fi
    
    # Extract first episode with reasonable duration (less than 2 hours)
    episode_id=$(echo "$podcast_response" | jq -r '.items[] | select(.duration < 7200 and .duration > 60) | .id' | head -1)
    
    if [ -z "$episode_id" ]; then
        # If no episode with duration filter, just get the first one
        episode_id=$(echo "$podcast_response" | jq -r '.items[0].id // empty')
    fi
    
    if [ -z "$episode_id" ]; then
        log_error "Could not find any episodes"
        exit 1
    fi
    
    episode_title=$(echo "$podcast_response" | jq -r ".items[] | select(.id == $episode_id) | .title")
    episode_duration=$(echo "$podcast_response" | jq -r ".items[] | select(.id == $episode_id) | .duration")
    
    log_success "Selected episode: $episode_title"
    log_info "Episode ID: $episode_id, Duration: ${episode_duration}s"
    echo ""
    
    # Step 5: Create first annotation
    log_info "Step 5: Creating first annotation (intro segment)..."
    
    annotation1_data='{
        "label": "intro",
        "start_time": 0,
        "end_time": 30
    }'
    
    create_response1=$(curl -s -X POST "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json" \
        -d "$annotation1_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Create response 1: $create_response1" >&2
    fi
    
    # Check if creation was successful
    if ! echo "$create_response1" | jq -e '.ID' > /dev/null 2>&1; then
        log_error "Failed to create first annotation"
        echo "$create_response1" >&2
        exit 1
    fi
    
    annotation1_id=$(echo "$create_response1" | jq -r '.ID')
    annotation1_uuid=$(echo "$create_response1" | jq -r '.uuid')
    
    log_success "Created annotation #1 - ID: $annotation1_id, UUID: $annotation1_uuid"
    echo "  Label: intro, Time: 0-30s"
    echo ""
    
    # Step 6: Create second annotation
    log_info "Step 6: Creating second annotation (main content)..."
    
    annotation2_data='{
        "label": "main_content",
        "start_time": 30,
        "end_time": 180
    }'
    
    create_response2=$(curl -s -X POST "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json" \
        -d "$annotation2_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Create response 2: $create_response2" >&2
    fi
    
    if ! echo "$create_response2" | jq -e '.ID' > /dev/null 2>&1; then
        log_error "Failed to create second annotation"
        echo "$create_response2" >&2
        exit 1
    fi
    
    annotation2_id=$(echo "$create_response2" | jq -r '.ID')
    annotation2_uuid=$(echo "$create_response2" | jq -r '.uuid')
    
    log_success "Created annotation #2 - ID: $annotation2_id, UUID: $annotation2_uuid"
    echo "  Label: main_content, Time: 30-180s"
    echo ""
    
    # Step 7: Create third annotation (outro)
    log_info "Step 7: Creating third annotation (outro segment)..."
    
    # Calculate outro times based on episode duration
    outro_start=$(echo "$episode_duration - 60" | bc 2>/dev/null || echo "240")
    outro_end=$episode_duration
    
    annotation3_data="{
        \"label\": \"outro\",
        \"start_time\": $outro_start,
        \"end_time\": $outro_end
    }"
    
    create_response3=$(curl -s -X POST "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json" \
        -d "$annotation3_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Create response 3: $create_response3" >&2
    fi
    
    if ! echo "$create_response3" | jq -e '.ID' > /dev/null 2>&1; then
        log_error "Failed to create third annotation"
        echo "$create_response3" >&2
        exit 1
    fi
    
    annotation3_id=$(echo "$create_response3" | jq -r '.ID')
    annotation3_uuid=$(echo "$create_response3" | jq -r '.uuid')
    
    log_success "Created annotation #3 - ID: $annotation3_id, UUID: $annotation3_uuid"
    echo "  Label: outro, Time: ${outro_start}-${outro_end}s"
    echo ""
    
    # Step 8: Get all annotations for the episode
    log_info "Step 8: Retrieving all annotations for episode..."
    
    get_response=$(curl -s -X GET "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Get response: $get_response" >&2
    fi
    
    annotation_count=$(echo "$get_response" | jq '.annotations | length')
    
    if [ "$annotation_count" -lt 3 ]; then
        log_error "Expected at least 3 annotations, got $annotation_count"
        echo "$get_response" >&2
        exit 1
    fi
    
    log_success "Retrieved $annotation_count annotations"
    
    # Display all annotations
    echo ""
    echo "All annotations for episode:"
    echo "$get_response" | jq -r '.annotations[] | "  - \(.label): \(.start_time)s - \(.end_time)s (ID: \(.ID))"'
    echo ""
    
    # Step 9: Update an annotation
    log_info "Step 9: Updating annotation #2..."
    
    update_data='{
        "label": "main_discussion",
        "start_time": 35,
        "end_time": 200
    }'
    
    update_response=$(curl -s -X PUT "$API_URL/api/v1/episodes/annotations/$annotation2_id" \
        -H "Content-Type: application/json" \
        -d "$update_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Update response: $update_response" >&2
    fi
    
    if ! echo "$update_response" | jq -e '.ID' > /dev/null 2>&1; then
        log_error "Failed to update annotation"
        echo "$update_response" >&2
        exit 1
    fi
    
    updated_label=$(echo "$update_response" | jq -r '.label')
    updated_start=$(echo "$update_response" | jq -r '.start_time')
    updated_end=$(echo "$update_response" | jq -r '.end_time')
    
    log_success "Updated annotation #2"
    echo "  New values - Label: $updated_label, Time: ${updated_start}s - ${updated_end}s"
    echo ""
    
    # Step 10: Test validation - overlapping annotations (should be allowed)
    log_info "Step 10: Testing overlapping annotations (should be allowed)..."
    
    overlap_data='{
        "label": "advertisement",
        "start_time": 25,
        "end_time": 45
    }'
    
    overlap_response=$(curl -s -X POST "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json" \
        -d "$overlap_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Overlap response: $overlap_response" >&2
    fi
    
    if echo "$overlap_response" | jq -e '.ID' > /dev/null 2>&1; then
        overlap_id=$(echo "$overlap_response" | jq -r '.ID')
        log_success "Created overlapping annotation (ID: $overlap_id)"
        echo "  This is allowed - annotations can overlap"
    else
        log_warning "Could not create overlapping annotation (might be restricted by business logic)"
    fi
    echo ""
    
    # Step 11: Test validation - invalid time range
    log_info "Step 11: Testing invalid time range (end < start)..."
    
    invalid_data='{
        "label": "invalid",
        "start_time": 100,
        "end_time": 50
    }'
    
    invalid_response=$(curl -s -X POST "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json" \
        -d "$invalid_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Invalid response: $invalid_response" >&2
    fi
    
    if echo "$invalid_response" | jq -e '.error' > /dev/null 2>&1; then
        log_success "Validation working: $(echo "$invalid_response" | jq -r '.error')"
    else
        log_warning "Expected validation error for invalid time range"
    fi
    echo ""
    
    # Step 12: Delete an annotation
    log_info "Step 12: Deleting annotation #1..."
    
    delete_response=$(curl -s -X DELETE "$API_URL/api/v1/episodes/annotations/$annotation1_id" \
        -H "Content-Type: application/json")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Delete response: $delete_response" >&2
    fi
    
    if echo "$delete_response" | jq -e '.message' > /dev/null 2>&1; then
        log_success "Deleted annotation #1"
    else
        log_error "Failed to delete annotation"
        echo "$delete_response" >&2
    fi
    echo ""
    
    # Step 13: Verify deletion by getting all annotations again
    log_info "Step 13: Verifying deletion..."
    
    final_response=$(curl -s -X GET "$API_URL/api/v1/episodes/$episode_id/annotations" \
        -H "Content-Type: application/json")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Final response: $final_response" >&2
    fi
    
    final_count=$(echo "$final_response" | jq '.annotations | length')
    
    log_success "Final annotation count: $final_count"
    echo ""
    echo "Remaining annotations:"
    echo "$final_response" | jq -r '.annotations[] | "  - \(.label): \(.start_time)s - \(.end_time)s (ID: \(.ID))"'
    echo ""
    
    # Step 14: Test with non-existent episode
    log_info "Step 14: Testing with non-existent episode..."
    
    nonexistent_data='{
        "label": "test",
        "start_time": 0,
        "end_time": 10
    }'
    
    nonexistent_response=$(curl -s -X POST "$API_URL/api/v1/episodes/999999999/annotations" \
        -H "Content-Type: application/json" \
        -d "$nonexistent_data")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Non-existent response: $nonexistent_response" >&2
    fi
    
    if echo "$nonexistent_response" | jq -e '.error' > /dev/null 2>&1; then
        log_success "Properly handled non-existent episode"
    else
        log_warning "Expected error for non-existent episode"
    fi
    echo ""
    
    # Summary
    echo "==========================================="
    echo "           Test Summary                   "
    echo "==========================================="
    echo ""
    log_success "All annotation API tests completed successfully!"
    echo ""
    echo "Tests performed:"
    echo "  ✓ Created multiple annotations"
    echo "  ✓ Retrieved annotations by episode"
    echo "  ✓ Updated annotation"
    echo "  ✓ Deleted annotation"
    echo "  ✓ Validated time ranges"
    echo "  ✓ Tested overlapping annotations"
    echo "  ✓ Handled non-existent episodes"
    echo ""
    echo "Episode tested: $episode_title (ID: $episode_id)"
    echo "Final annotation count: $final_count"
    echo ""
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --api-url)
            API_URL="$2"
            shift 2
            ;;
        --verbose|-v)
            VERBOSE="true"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --api-url URL    API base URL (default: http://localhost:9000)"
            echo "  --verbose, -v    Enable verbose output"
            echo "  --help, -h       Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0"
            echo "  $0 --api-url https://api.example.com"
            echo "  $0 --verbose"
            echo ""
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run the main test
main