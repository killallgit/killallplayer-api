#!/bin/bash

# Upgrade user to admin permissions

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

echo "🔧 Upgrading user to admin permissions"
echo "User: $USER_EMAIL"
echo

# Get user ID
USER_DATA=$(curl -s -X GET "$SUPABASE_URL/auth/v1/admin/users" \
    -H "Authorization: Bearer $SUPABASE_API_KEY" \
    -H "apikey: $SUPABASE_API_KEY" \
    -H "Content-Type: application/json")

if command -v jq &> /dev/null; then
    USER_ID=$(echo "$USER_DATA" | jq -r ".users[] | select(.email == \"$USER_EMAIL\") | .id")
else
    USER_ID=$(echo "$USER_DATA" | grep -A 10 -B 10 "$USER_EMAIL" | grep '"id"' | head -1 | sed 's/.*"id": *"\([^"]*\)".*/\1/')
fi

if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
    echo "❌ User not found: $USER_EMAIL"
    exit 1
fi

echo "✅ Found user ID: $USER_ID"

# Update user with admin permissions
echo "🔧 Adding admin permissions..."
UPDATE_RESPONSE=$(curl -s -X PUT "$SUPABASE_URL/auth/v1/admin/users/$USER_ID" \
    -H "Authorization: Bearer $SUPABASE_API_KEY" \
    -H "apikey: $SUPABASE_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "app_metadata": {
            "permissions": ["podcasts:admin"],
            "role": "admin"
        }
    }')

if [[ $? -ne 0 ]]; then
    echo "❌ Failed to update user permissions"
    exit 1
fi

echo "✅ User upgraded to admin"

# Verify the permissions
VERIFY_RESPONSE=$(curl -s -X GET "$SUPABASE_URL/auth/v1/admin/users/$USER_ID" \
    -H "Authorization: Bearer $SUPABASE_API_KEY" \
    -H "apikey: $SUPABASE_API_KEY" \
    -H "Content-Type: application/json")

echo "📋 Updated User Details:"
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
echo "🎉 Admin upgrade complete!"
echo "The user now has 'podcasts:admin' permission which includes all read/write access."