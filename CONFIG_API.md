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

## Generic Command Runner (Whitelisted)

In addition to the long-running `services`, `conf.json` may declare a
whitelist of one-shot commands that the frontend / admin scripts can invoke
on demand. Each command is identified by an `id`. Callers cannot supply
arbitrary arguments — they may only reference a configured `id`, and the
binary path and arguments are taken verbatim from the configuration.

### Configuration

Add an `allowed_commands` array to `conf.json` alongside `services`:

```json
{
  "services": [ ... ],
  "allowed_commands": [
    {
      "id": "mpx-watchdog-status",
      "cmd": "/usr/local/bin/mpx-watchdog",
      "args": ["ctl", "status", "--json"],
      "timeout_seconds": 5,
      "description": "Get keepalived watchdog status"
    },
    {
      "id": "mpx-watchdog-promote",
      "cmd": "/usr/local/bin/mpx-watchdog",
      "args": ["ctl", "promote"],
      "timeout_seconds": 30,
      "description": "Promote node to MASTER"
    }
  ]
}
```

Field rules:

- `id` — unique identifier referenced by callers (MQTT payload / HTTP path).
- `cmd` — absolute path of the binary to execute.
- `args` — fixed list of arguments. Callers cannot append or override them.
- `timeout_seconds` — optional. Default `10`, clamped to `[1, 300]`.
- `description` — free-form human-readable note (not used by the runner).

If `allowed_commands` is absent the runner is effectively disabled — every
incoming request gets `command not allowed: <id>`.

### MQTT Contract

Topics (the `<exec_topic>` base is `exec` by default; it can be overridden
through `mqtt.exec_cmd_topic` / `mqtt.exec_cmd_data_topic`):

- Request (subscribed by exec-api): `exec/cmd/<client_id>`
- Response (published by exec-api): `exec/cmd/data/<client_id>`
  - `Retained = false`, `QoS = 0`.

Request payload:

```json
{
  "request_id": "abc-123",
  "id": "mpx-watchdog-status"
}
```

- `request_id` — free-form string the caller uses to match responses. Echoed back unchanged.
- `id` — must match a configured `allowed_commands[].id`.

Response payload:

```json
{
  "request_id": "abc-123",
  "id": "mpx-watchdog-status",
  "exit_code": 0,
  "stdout": "{\"state\":\"MASTER\"}\n",
  "stderr": "",
  "duration_ms": 42,
  "error": "",
  "timestamp": "2026-05-13T10:11:12Z"
}
```

Error cases:

- Unknown / non-whitelisted `id` → `exit_code = -1`, `error = "command not allowed: <id>"`, empty `stdout`/`stderr`.
- Timeout → `exit_code = -1`, `error = "timeout after Ns"`, partial output retained.
- Failure to spawn (binary missing, permission denied, …) → `exit_code = -1`, `error = "<message>"`.
- Malformed payload → `exit_code = -1`, `error = "invalid payload: ..."`.
- Process started and exited normally with non-zero status → real `exit_code`, `error` empty.

### HTTP Contract

```
POST /:ep/cmd/:id
```

Convenience endpoint for debugging / admin scripts. Auth is handled by the
existing middleware chain (same as every other route).

- `:ep` — endpoint name (used by the config-fetching helpers, e.g. the client_id).
- `:id` — id of the allowed command.

Request body (optional):

```json
{ "request_id": "abc-123" }
```

Response body matches the MQTT response shape. Status codes:

- `200` — command ran (regardless of exit code).
- `400` — malformed JSON body.
- `404` — `id` not present in `allowed_commands`.
- `500` — system-level failure (config load error, exec error, timeout).

Example:

```bash
curl -X POST "http://localhost:8080/testcap/cmd/mpx-watchdog-status" \
  -H "Content-Type: application/json" \
  -d '{"request_id":"debug-1"}'
```
