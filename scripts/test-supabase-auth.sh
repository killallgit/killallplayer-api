#!/bin/bash

# Test real Supabase authentication
# This script logs in with Supabase and tests the JWT

set -e  # Exit on any error

# Configuration
SUPABASE_URL="https://japdtgkuznxdgdvaxykr.supabase.co"
API_URL=${API_URL:-"http://localhost:9000"}

# Load credentials from environment or .env file
if [[ -f .env ]]; then
    source .env
fi

if [[ -z "$SUPABASE_ADMIN_USER" || -z "$SUPABASE_ADMIN_PASSWORD" ]]; then
    echo "❌ Error: SUPABASE_ADMIN_USER and SUPABASE_ADMIN_PASSWORD not found in .env file"
    exit 1
fi

echo "🔐 Testing Real Supabase Authentication"
echo "======================================"
echo "User: $SUPABASE_ADMIN_USER"
echo "API: $API_URL"
echo

# Step 1: Login with Supabase
echo "1️⃣ Logging in with Supabase..."
LOGIN_RESPONSE=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" \
    -H "Content-Type: application/json" \
    -H "apikey: $SUPABASE_ANON_KEY" \
    -d '{
        "email": "'$SUPABASE_ADMIN_USER'",
        "password": "'$SUPABASE_ADMIN_PASSWORD'"
    }')

if [[ $? -ne 0 ]]; then
    echo "❌ Failed to login with Supabase"
    exit 1
fi

# Check for errors in response
if echo "$LOGIN_RESPONSE" | grep -q '"error"'; then
    echo "❌ Supabase login failed:"
    echo "$LOGIN_RESPONSE" | jq .
    exit 1
fi

echo "✅ Supabase login successful"

# Extract the access token
if command -v jq &> /dev/null; then
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.access_token')
    USER_ID=$(echo "$LOGIN_RESPONSE" | jq -r '.user.id')
    USER_EMAIL=$(echo "$LOGIN_RESPONSE" | jq -r '.user.email')
else
    # Fallback without jq
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    USER_ID=$(echo "$LOGIN_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    USER_EMAIL=$(echo "$LOGIN_RESPONSE" | grep -o '"email":"[^"]*"' | cut -d'"' -f4)
fi

if [[ -z "$ACCESS_TOKEN" || "$ACCESS_TOKEN" == "null" ]]; then
    echo "❌ Failed to extract access token"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

echo "📄 Token Info:"
echo "  User ID: $USER_ID"
echo "  Email: $USER_EMAIL"
echo "  Token: ${ACCESS_TOKEN:0:20}..."
echo

# Step 2: Test /me endpoint with Supabase JWT
echo "2️⃣ Testing /me endpoint with Supabase JWT..."
ME_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/me" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

echo "Response:"
if command -v jq &> /dev/null; then
    echo "$ME_RESPONSE" | jq .
else
    echo "$ME_RESPONSE"
fi
echo

# Step 3: Test protected API endpoint
echo "3️⃣ Testing protected API endpoint with Supabase JWT..."
SEARCH_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/search" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"query": "technology", "limit": 1}')

echo "Search Response:"
if command -v jq &> /dev/null; then
    echo "$SEARCH_RESPONSE" | jq .
else
    echo "$SEARCH_RESPONSE"
fi
echo

# Step 4: Verify JWT contents (decode without verification)
echo "4️⃣ JWT Token Analysis:"
if command -v jq &> /dev/null; then
    # Decode the JWT payload (second part)
    JWT_PAYLOAD=$(echo "$ACCESS_TOKEN" | cut -d. -f2)
    # Add padding if needed
    case $((${#JWT_PAYLOAD} % 4)) in
        2) JWT_PAYLOAD="${JWT_PAYLOAD}==" ;;
        3) JWT_PAYLOAD="${JWT_PAYLOAD}=" ;;
    esac

    echo "JWT Claims:"
    echo "$JWT_PAYLOAD" | base64 -d 2>/dev/null | jq . || echo "Could not decode JWT payload"
fi

echo
echo "🎉 Supabase authentication test complete!"
echo
echo "Summary:"
echo "- ✅ Supabase login: Success"
echo "- 🔍 JWT validation: Check the responses above"
echo "- 🎯 API access: Check if search worked"