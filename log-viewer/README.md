# AI Studio Proxy - Log Viewer

Interactive web-based log viewer for monitoring all requests, responses, and errors in real-time.

## Features

- Real-time log streaming (auto-refresh every 2 seconds)
- Verbose request/response logging with full headers and bodies
- Filter by log level (ERROR, WARN, INFO, DEBUG)
- Search logs by message or data content
- Click any log entry to view full details including JSON data
- Download logs as JSON
- System health monitoring (active users, connections, buffer size)
- Dark theme optimized for monitoring

## Access

Once the container is running, access the log viewer at:

**http://localhost:5345/logs-ui/**

## API Endpoints

The Go proxy exposes the following endpoints:

- `GET /api/logs` - Returns all buffered logs (last 1000 entries)
- `GET /api/health` - System health check with connection stats
- `GET /logs-ui/` - Static React app for the log viewer

## Log Levels

- **ERROR**: Critical errors, network failures, authentication issues
- **WARN**: Warnings that don't prevent operation
- **INFO**: Normal operations (requests, responses, connections)
- **DEBUG**: Detailed debugging information

## Debugging Tool Calling

When you make API requests with tool calling, the logs will show:

1. Full request body including tool definitions
2. All headers sent to Gemini API
3. Complete response body with error details
4. Status codes and error messages

This is extremely useful for debugging the 400 error:
`{"error":{"code":400,"message":"Request contains an invalid argument.","status":"INVALID_ARGUMENT"}}`

## Development

### Build locally

```bash
cd log-viewer
npm install
npm run dev  # Development server on http://localhost:3000
npm run build  # Production build to dist/
```

### Update in Docker

After making changes to the React app:

```bash
docker compose build --no-cache
docker compose up -d
```
