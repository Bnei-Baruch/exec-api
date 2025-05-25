#!/bin/bash

# Test script for configuration management API
BASE_URL="http://localhost:8080"

echo "=== Testing Configuration Management API ==="

echo "1. Getting current configuration:"
curl -X GET "$BASE_URL/config" | jq .

echo -e "\n2. Updating arguments for 'sdi' service:"
curl -X PUT "$BASE_URL/config/service/sdi/args" \
  -H "Content-Type: application/json" \
  -d '{"args":["-progress","stat_sdi.log","-hide_banner", "-y", "-f", "lavfi", "-re", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null", "-t", "10"]}' | jq .

echo -e "\n3. Adding new service:"
curl -X POST "$BASE_URL/config/service" \
  -H "Content-Type: application/json" \
  -d '{"id":"python","name":"python3","description":"Python whisper service","args":["whisper_fullfile_cli.py","--txt","--srt", "--input","tmp/input.mp3"]}' | jq .

echo -e "\n4. Checking updated configuration:"
curl -X GET "$BASE_URL/config" | jq .

echo -e "\n5. Deleting 'python' service:"
curl -X DELETE "$BASE_URL/config/service/python" | jq .

echo -e "\n6. Final configuration check:"
curl -X GET "$BASE_URL/config" | jq .

echo -e "\n=== Testing completed ===" 