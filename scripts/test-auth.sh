#!/bin/bash

# Test authentication endpoints

API_URL=${API_URL:-"http://localhost:9000"}
DEV_TOKEN="foobarbaz"

echo "Testing Supabase Authentication Endpoints"
echo "========================================="
echo

echo "1. Testing dev-login endpoint..."
echo "   POST $API_URL/api/v1/auth/dev-login"
DEV_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/dev-login")
echo "$DEV_RESPONSE" | jq .
echo

echo "2. Testing /me endpoint with dev token..."
echo "   GET $API_URL/api/v1/me"
ME_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/me" \
  -H "Authorization: Bearer $DEV_TOKEN")
echo "$ME_RESPONSE" | jq .
echo

echo "3. Testing protected endpoint without token (should fail)..."
echo "   GET $API_URL/api/v1/me"
curl -s -X GET "$API_URL/api/v1/me" | jq .
echo

echo "4. Testing protected API endpoint with dev token..."
echo "   POST $API_URL/api/v1/search"
SEARCH_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/search" \
  -H "Authorization: Bearer $DEV_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "technology", "limit": 2}')
echo "$SEARCH_RESPONSE" | jq .
echo

echo "================================"
echo "Dev Token for easy testing:"
echo "$DEV_TOKEN"
echo
echo "Use it in curl like this:"
echo "curl -H \"Authorization: Bearer $DEV_TOKEN\" $API_URL/api/v1/me"