# Configuration Management API

This document describes new endpoints for managing service configuration in the `conf.json` file.

## Endpoints

### 1. Get Current Configuration

**GET** `/config`

Returns the current configuration of all services.

**Response:**
```json
{
  "services": [
    {
      "id": "sdi",
      "name": "ffmpeg",
      "description": "",
      "args": ["-progress", "stat_sdi.log", "-hide_banner", "-y", "-f", "lavfi", "-re", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null"]
    }
  ]
}
```

### 2. Update Service Arguments

**PUT** `/config/service/:id/args`

Updates arguments for the specified service.

**Parameters:**
- `id` - service identifier

**Request Body:**
```json
{
  "args": ["new", "list", "of", "arguments"]
}
```

**Example:**
```bash
curl -X PUT "http://localhost:8080/config/service/sdi/args" \
  -H "Content-Type: application/json" \
  -d '{"args":["-progress","stat_sdi.log","-hide_banner", "-y", "-f", "lavfi", "-re", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null", "-t", "10"]}'
```

**Response:**
```json
{
  "result": "success",
  "service": {
    "id": "sdi",
    "name": "ffmpeg",
    "description": "",
    "args": ["-progress", "stat_sdi.log", "-hide_banner", "-y", "-f", "lavfi", "-re", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null", "-t", "10"]
  }
}
```

### 3. Add New Service

**POST** `/config/service`

Adds a new service to the configuration.

**Request Body:**
```json
{
  "id": "python",
  "name": "python3",
  "description": "Python whisper service",
  "args": ["whisper_fullfile_cli.py", "--txt", "--srt", "--input", "tmp/input.mp3"]
}
```

**Example:**
```bash
curl -X POST "http://localhost:8080/config/service" \
  -H "Content-Type: application/json" \
  -d '{"id":"python","name":"python3","description":"Python whisper service","args":["whisper_fullfile_cli.py","--txt","--srt", "--input","tmp/input.mp3"]}'
```

**Response:**
```json
{
  "result": "success",
  "service": {
    "id": "python",
    "name": "python3",
    "description": "Python whisper service",
    "args": ["whisper_fullfile_cli.py", "--txt", "--srt", "--input", "tmp/input.mp3"]
  }
}
```

### 4. Delete Service

**DELETE** `/config/service/:id`

Removes a service from the configuration.

**Parameters:**
- `id` - service identifier

**Example:**
```bash
curl -X DELETE "http://localhost:8080/config/service/python"
```

**Response:**
```json
{
  "result": "success",
  "message": "Service deleted"
}
```

## Error Codes

- `400 Bad Request` - invalid request format or missing required fields
- `404 Not Found` - service not found
- `409 Conflict` - service with this ID already exists (when adding)
- `500 Internal Server Error` - server error when reading/writing configuration

## Usage Examples

### Updating arguments for whisper service

```bash
# Add new whisper service
curl -X POST "http://localhost:8080/config/service" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "whisper",
    "name": "python3",
    "description": "Whisper transcription service",
    "args": ["whisper_fullfile_cli.py", "--txt", "--srt", "--input", "tmp/input.mp3"]
  }'

# Update arguments to add language
curl -X PUT "http://localhost:8080/config/service/whisper/args" \
  -H "Content-Type: application/json" \
  -d '{
    "args": ["whisper_fullfile_cli.py", "--txt", "--srt", "--language", "ru", "--input", "tmp/input.mp3"]
  }'
```

### Managing ffmpeg services

```bash
# Update arguments to add time limit
curl -X PUT "http://localhost:8080/config/service/sdi/args" \
  -H "Content-Type: application/json" \
  -d '{
    "args": ["-progress", "stat_sdi.log", "-hide_banner", "-y", "-f", "lavfi", "-re", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null", "-t", "3600"]
  }'
```

## Security

⚠️ **Important:** These endpoints allow modifying server configuration. Make sure access to them is restricted to authorized users only.

## Automatic Application of Changes

After changing configuration through the API, changes are saved to the `conf.json` file. To apply new settings to already running services, they may need to be restarted through the corresponding endpoints (`/stop` and `/start`). 