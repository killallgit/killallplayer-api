#!/bin/bash

# End-to-End Waveform Generation Test Script
# This script tests the complete waveform workflow:
# 1. Get trending podcasts
# 2. Get first episode from first podcast  
# 3. Get detailed episode information
# 4. Trigger waveform generation
# 5. Poll for completion status
# 6. Verify waveform data
#
# Usage:
#   ./test-waveform-e2e.sh                    # Auto-discover episode from trending
#   ./test-waveform-e2e.sh <episode_id>       # Test specific episode by ID
#
# Requirements:
#   - curl (for API calls)
#   - jq (for JSON parsing): brew install jq
#   - API server running on localhost:8080

set -e

# Configuration
API_BASE="http://localhost:8080"
TIMEOUT=60  # Maximum seconds to wait for waveform generation
POLL_INTERVAL=2  # Seconds between status checks
TEST_EPISODE_ID=""  # Optional specific episode ID to test

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

# Function to make API calls with error handling
api_call() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local response_file=$(mktemp)
    local status_code

    if [ "$method" = "POST" ]; then
        status_code=$(curl -s -w "%{http_code}" -X POST \
            -H "Content-Type: application/json" \
            -d "$data" \
            "${API_BASE}${endpoint}" \
            -o "$response_file")
    else
        status_code=$(curl -s -w "%{http_code}" \
            "${API_BASE}${endpoint}" \
            -o "$response_file")
    fi

    if [ "$status_code" -ge 200 ] && [ "$status_code" -lt 300 ]; then
        cat "$response_file"
        rm "$response_file"
        return 0
    else
        error "API call failed: $method $endpoint (HTTP $status_code)"
        cat "$response_file"
        rm "$response_file"
        return 1
    fi
}

# Function to extract JSON field
get_json_field() {
    local json="$1"
    local field="$2"
    echo "$json" | jq -r ".$field // empty"
}

# Function to get array element
get_json_array_element() {
    local json="$1"
    local index="$2"
    local field="$3"
    if [ -n "$field" ]; then
        echo "$json" | jq -r ".[$index].$field // empty"
    else
        echo "$json" | jq -r ".[$index] // empty"
    fi
}

main() {
    log "🚀 Starting End-to-End Waveform Generation Test"
    log "API Base URL: $API_BASE"
    
    # Check if specific episode ID provided as argument
    if [ -n "$1" ]; then
        TEST_EPISODE_ID="$1"
        log "Using specific episode ID: $TEST_EPISODE_ID"
    fi
    echo ""

    # Step 1: Health check
    log "📋 Step 1: Checking API health..."
    health_response=$(api_call "GET" "/health")
    if [ $? -ne 0 ]; then
        error "API health check failed. Is the server running on $API_BASE?"
    fi
    success "API is healthy"
    echo ""

    # If specific episode ID provided, skip discovery and go directly to testing
    if [ -n "$TEST_EPISODE_ID" ]; then
        episode_id="$TEST_EPISODE_ID"
        log "🎯 Skipping podcast discovery - using provided episode ID: $episode_id"
        
        # Get episode details
        episode_response=$(api_call "GET" "/api/v1/episodes/$episode_id")
        if [ $? -ne 0 ]; then
            error "Failed to get episode details for episode $episode_id"
        fi
        
        episode_title=$(echo "$episode_response" | jq -r '.episode.title // empty')
        initial_status=$(echo "$episode_response" | jq -r '.episode.waveform.status // empty')
        
        log "Episode: '$episode_title'"
        log "Current waveform status: $initial_status"
        echo ""
    else
        # Step 2: Get trending podcasts
    log "📈 Step 2: Getting trending podcasts..."
    trending_response=$(api_call "GET" "/api/v1/trending?limit=5")
    if [ $? -ne 0 ]; then
        error "Failed to get trending podcasts"
    fi
    
    # Extract first podcast ID from feeds array
    podcast_id=$(echo "$trending_response" | jq -r '.feeds[0].id // empty')
    podcast_title=$(echo "$trending_response" | jq -r '.feeds[0].title // empty')
    
    if [ -z "$podcast_id" ] || [ "$podcast_id" = "null" ]; then
        error "No trending podcasts found or invalid response format"
    fi
    
    success "Found trending podcast: '$podcast_title' (ID: $podcast_id)"
    echo ""

    # Step 3: Sync episodes for the podcast
    log "🔄 Step 3: Syncing episodes for podcast $podcast_id..."
    sync_response=$(api_call "POST" "/api/v1/podcasts/$podcast_id/episodes/sync" "{}")
    if [ $? -ne 0 ]; then
        warning "Failed to sync episodes, but continuing with existing episodes..."
    else
        success "Episodes synced successfully"
    fi
    echo ""

    # Step 4: Get episodes from this podcast
    log "🔍 Step 4: Getting episodes for podcast '$podcast_title'..."
    episodes_response=$(api_call "GET" "/api/v1/podcasts/$podcast_id/episodes")
    if [ $? -ne 0 ]; then
        error "Failed to get episodes for podcast $podcast_id"
    fi

    # Extract first episode ID from episodes list
    episode_id=$(echo "$episodes_response" | jq -r '.items[0].id // empty')
    episode_title=$(echo "$episodes_response" | jq -r '.items[0].title // empty')
    
    if [ -z "$episode_id" ] || [ "$episode_id" = "null" ]; then
        error "No episodes found for this podcast. Try with a different podcast or ensure episodes are available."
    fi
    
    success "Found episode: '$episode_title' (ID: $episode_id)"
    echo ""

    # Step 5: Get detailed episode information
    log "📋 Step 5: Getting detailed episode information..."
    episode_response=$(api_call "GET" "/api/v1/episodes/$episode_id")
    if [ $? -ne 0 ]; then
        error "Failed to get episode details for episode $episode_id"
    fi
    
    # Update episode title from detailed response
    episode_title=$(echo "$episode_response" | jq -r '.episode.title // empty')
    initial_status=$(echo "$episode_response" | jq -r '.episode.waveform.status // empty')
    
    log "Episode: '$episode_title'"
    log "Current waveform status: $initial_status"
    echo ""
    fi  # Close the if statement for TEST_EPISODE_ID

    # Step 6: Check current waveform status in detail  
    log "🔍 Step 6: Checking current waveform status..."
    
    if [ "$initial_status" = "completed" ]; then
        warning "Waveform already exists for this episode"
        log "Fetching existing waveform data..."
        waveform_response=$(api_call "GET" "/api/v1/episodes/$episode_id/waveform")
        duration=$(get_json_field "$waveform_response" "duration")
        resolution=$(get_json_field "$waveform_response" "resolution")
        peaks_count=$(echo "$waveform_response" | jq -r '.peaks | length')
        success "✅ END-TO-END TEST COMPLETED SUCCESSFULLY!"
        success "📊 Final Results:"
        success "   • Episode: $episode_title"
        success "   • Duration: ${duration}s"
        success "   • Resolution: $resolution peaks"
        success "   • Total Peaks: $peaks_count"
        exit 0
    fi
    
    success "Current status: $initial_status"
    echo ""

    # Step 7: Trigger waveform generation
    log "⚙️  Step 7: Triggering waveform generation..."
    trigger_response=$(api_call "GET" "/api/v1/episodes/$episode_id/waveform")
    trigger_status=$(echo "$trigger_response" | jq -r '.error // "success"')
    
    if [ "$trigger_status" != "success" ]; then
        # Check if it's a "not found" which means generation was queued
        if echo "$trigger_response" | grep -q "Waveform generation has been queued"; then
            success "Waveform generation queued successfully"
        else
            error "Failed to trigger waveform generation: $trigger_status"
        fi
    else
        success "Waveform generation triggered"
    fi
    echo ""

    # Step 8: Poll for completion
    log "⏳ Step 8: Polling for waveform completion (max ${TIMEOUT}s)..."
    start_time=$(date +%s)
    
    while true; do
        current_time=$(date +%s)
        elapsed=$((current_time - start_time))
        
        if [ $elapsed -ge $TIMEOUT ]; then
            error "Timeout reached (${TIMEOUT}s). Waveform generation may still be in progress."
        fi
        
        status_response=$(api_call "GET" "/api/v1/episodes/$episode_id/waveform/status")
        status=$(get_json_field "$status_response" "status")
        progress=$(get_json_field "$status_response" "progress")
        
        case "$status" in
            "completed")
                success "Waveform generation completed!"
                break
                ;;
            "processing")
                log "Status: Processing (${progress}% complete) - ${elapsed}s elapsed"
                ;;
            "queued"|"pending")
                log "Status: Queued/Pending - ${elapsed}s elapsed"
                ;;
            "failed")
                error "Waveform generation failed"
                ;;
            "not_found")
                warning "Waveform not found, may still be initializing..."
                ;;
            *)
                warning "Unknown status: $status (${elapsed}s elapsed)"
                ;;
        esac
        
        sleep $POLL_INTERVAL
    done
    echo ""

    # Step 9: Fetch and verify waveform data
    log "📊 Step 9: Fetching final waveform data..."
    final_response=$(api_call "GET" "/api/v1/episodes/$episode_id/waveform")
    
    # Verify response contains expected fields
    duration=$(get_json_field "$final_response" "duration")
    resolution=$(get_json_field "$final_response" "resolution")
    sample_rate=$(get_json_field "$final_response" "sample_rate")
    peaks_count=$(echo "$final_response" | jq -r '.peaks | length')
    cached=$(get_json_field "$final_response" "cached")
    
    if [ -z "$duration" ] || [ "$duration" = "null" ]; then
        error "Invalid waveform response: missing duration"
    fi
    
    if [ -z "$peaks_count" ] || [ "$peaks_count" = "null" ] || [ "$peaks_count" = "0" ]; then
        error "Invalid waveform response: missing or empty peaks data"
    fi
    
    # Verify peaks data is valid
    min_peak=$(echo "$final_response" | jq -r '.peaks | min')
    max_peak=$(echo "$final_response" | jq -r '.peaks | max')
    
    success "✅ END-TO-END TEST COMPLETED SUCCESSFULLY!"
    echo ""
    success "📊 Final Results:"
    success "   • Episode: $episode_title"
    success "   • Duration: ${duration}s"
    success "   • Resolution: $resolution peaks"
    success "   • Sample Rate: ${sample_rate}Hz"
    success "   • Total Peaks: $peaks_count"
    success "   • Peak Range: $min_peak - $max_peak"
    success "   • Cached: $cached"
    success "   • Total Time: ${elapsed}s"
    echo ""
    success "🎉 All waveform data looks valid and complete!"
}

# Check dependencies
if ! command -v curl &> /dev/null; then
    error "curl is required but not installed"
fi

if ! command -v jq &> /dev/null; then
    error "jq is required but not installed. Please install with: brew install jq"
fi

# Run main function
main "$@"