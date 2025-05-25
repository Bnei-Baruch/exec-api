# Exec API

A Go-based API server for executing and managing system commands and services.

## Features

- Execute and manage system commands
- Service lifecycle management (start/stop/status)
- Progress monitoring and reporting
- File upload and management
- **Configuration Management API** - New feature for editing service arguments

## Configuration Management

The project now includes a comprehensive API for managing service configuration in `conf.json`:

### New Endpoints

- `GET /config` - Get current configuration
- `PUT /config/service/:id/args` - Update service arguments
- `POST /config/service` - Add new service
- `DELETE /config/service/:id` - Delete service

### Example Usage

```bash
# Get current configuration
curl -X GET "http://localhost:8080/config"

# Update arguments for a service
curl -X PUT "http://localhost:8080/config/service/sdi/args" \
  -H "Content-Type: application/json" \
  -d '{"args":["-progress","stat_sdi.log","-hide_banner", "-y", "-f", "lavfi", "-re", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null", "-t", "10"]}'

# Add new service
curl -X POST "http://localhost:8080/config/service" \
  -H "Content-Type: application/json" \
  -d '{"id":"python","name":"python3","description":"Python whisper service","args":["whisper_fullfile_cli.py","--txt","--srt", "--input","tmp/input.mp3"]}'
```

## Testing

Use the provided test script to test the configuration management API:

```bash
./test_config_api.sh
```

## Documentation

See `CONFIG_API.md` for detailed documentation of the configuration management endpoints.

## Building

```bash
go build -o exec-api .
```

## Running

```bash
./exec-api
```

The server will start on port 8080 by default (configurable via `config.toml`).
