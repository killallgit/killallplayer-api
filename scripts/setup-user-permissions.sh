#!/bin/bash

# Setup user permissions in Supabase
# This script adds podcast permissions to your test user

set -e  # Exit on any error

# Configuration
USER_EMAIL="killallplayer@dot-gov.com"
SUPABASE_URL="https://japdtgkuznxdgdvaxykr.supabase.co"

# Load API key from environment or .env file
if [[ -f .env ]]; then
    source .env
fi

if [[ -z "$SUPABASE_API_KEY" ]]; then
    echo "❌ Error: SUPABASE_API_KEY not found in environment or .env file"
    exit 1
fi

echo "🚀 Setting up permissions for user: $USER_EMAIL"
echo "📍 Supabase URL: $SUPABASE_URL"
echo

# Step 1: Get all users to find the user ID
echo "1️⃣ Looking up user ID..."
USER_DATA=$(curl -s -X GET "$SUPABASE_URL/auth/v1/admin/users" \
    -H "Authorization: Bearer $SUPABASE_API_KEY" \
    -H "apikey: $SUPABASE_API_KEY" \
    -H "Content-Type: application/json")

if [[ $? -ne 0 ]]; then
    echo "❌ Failed to fetch users"
    exit 1
fi

# Extract user ID (using jq if available, otherwise manual parsing)
if command -v jq &> /dev/null; then
    USER_ID=$(echo "$USER_DATA" | jq -r ".users[] | select(.email == \"$USER_EMAIL\") | .id")
else
    # Fallback: manual parsing (less reliable but works without jq)
    USER_ID=$(echo "$USER_DATA" | grep -A 10 -B 10 "$USER_EMAIL" | grep '"id"' | head -1 | sed 's/.*"id": *"\([^"]*\)".*/\1/')
fi

if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
    echo "❌ User not found: $USER_EMAIL"
    echo "Available users:"
    if command -v jq &> /dev/null; then
        echo "$USER_DATA" | jq -r '.users[].email'
    else
        echo "$USER_DATA" | grep '"email"' | sed 's/.*"email": *"\([^"]*\)".*/\1/'
    fi
    exit 1
fi

echo "✅ Found user ID: $USER_ID"
echo

# Step 2: Update user with permissions
echo "2️⃣ Adding permissions to user..."
UPDATE_RESPONSE=$(curl -s -X PUT "$SUPABASE_URL/auth/v1/admin/users/$USER_ID" \
    -H "Authorization: Bearer $SUPABASE_API_KEY" \
    -H "apikey: $SUPABASE_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "app_metadata": {
            "permissions": ["podcasts:read", "podcasts:write"],
            "role": "user"
        }
    }')

if [[ $? -ne 0 ]]; then
    echo "❌ Failed to update user permissions"
    exit 1
fi

echo "✅ Update response received"
echo

# Step 3: Verify the permissions were added
echo "3️⃣ Verifying permissions..."
VERIFY_RESPONSE=$(curl -s -X GET "$SUPABASE_URL/auth/v1/admin/users/$USER_ID" \
    -H "Authorization: Bearer $SUPABASE_API_KEY" \
    -H "apikey: $SUPABASE_API_KEY" \
    -H "Content-Type: application/json")

if [[ $? -ne 0 ]]; then
    echo "❌ Failed to verify user permissions"
    exit 1
fi

echo "✅ Verification complete"
echo

# Show the results
echo "📋 User Details:"
if command -v jq &> /dev/null; then
    echo "$VERIFY_RESPONSE" | jq '{
        id: .id,
        email: .email,
        app_metadata: .app_metadata
    }'
else
    echo "Email: $USER_EMAIL"
    echo "ID: $USER_ID"
    echo "App Metadata:"
    echo "$VERIFY_RESPONSE" | grep -A 10 '"app_metadata"'
fi

echo
echo "🎉 Success! User permissions have been set up."
echo
echo "Now you can:"
echo "1. Login with this user in your app"
echo "2. Get a JWT token from Supabase"
echo "3. Use that token with your podcast API"
echo
echo "Next steps:"
echo "- Make sure your KILLALL_SUPABASE_JWT_SECRET is set correctly"
echo "- Test authentication with your app"