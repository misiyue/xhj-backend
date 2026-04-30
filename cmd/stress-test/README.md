# Stress Test Tool

A comprehensive stress testing tool for the IM (Instant Messaging) system. This tool simulates real-world operations including user creation, friend requests, group creation, and message sending.

## Features

- **User Creation Test**: Create multiple users simultaneously to test registration system scalability
- **Friend Request Test**: Simulate friend request operations at scale
- **Group Chat Test**: Create groups and simulate concurrent message sending
- **Mixed Operations Test**: Run a combination of different operations to simulate real-world usage
- **Database Cleanup**: Clean up test data and reset the database

## Installation

```bash
cd backend/cmd/stress-test
go build -o stress-test
```

## Usage

### Interactive Mode

Run the tool in interactive mode to be prompted for configuration:

```bash
./stress-test interactive
```

You'll be asked to:
1. Enter HTTP endpoint (default: http://localhost:9503)
2. Enter WebSocket endpoint (default: ws://localhost:9501/wss)
3. Select a test scenario
4. Configure test parameters

### Command-Line Mode

Run predefined test scenarios directly from the command line:

#### User Creation Test

Create a specified number of users:

```bash
./stress-test scenario user-creation --count 100 --http http://localhost:9503
```

Options:
- `--count`: Number of users to create (default: 100)
- `--http`: HTTP endpoint (default: http://localhost:9503)

#### Friend Request Test

Test friend request operations:

```bash
./stress-test scenario friend-requests --users 50 --requests 10 --http http://localhost:9503
```

Options:
- `--users`: Number of users to create (default: 50)
- `--requests`: Number of friend requests per user (default: 10)
- `--http`: HTTP endpoint (default: http://localhost:9503)

#### Group Chat Test

Test group chat messaging:

```bash
./stress-test scenario group-chat \
  --groups 10 \
  --members 20 \
  --messages 100 \
  --http http://localhost:9503 \
  --ws ws://localhost:9501/wss
```

Options:
- `--groups`: Number of groups to create (default: 10)
- `--members`: Number of members per group (default: 20)
- `--messages`: Number of messages per member (default: 100)
- `--http`: HTTP endpoint (default: http://localhost:9503)
- `--ws`: WebSocket endpoint (default: ws://localhost:9501/wss)

#### Mixed Operations Test

Run a combination of different operations:

```bash
./stress-test scenario mixed-operations \
  --duration 60 \
  --concurrency 50 \
  --http http://localhost:9503 \
  --ws ws://localhost:9501/wss
```

Options:
- `--duration`: Test duration in seconds (default: 60)
- `--concurrency`: Number of concurrent users (default: 50)
- `--http`: HTTP endpoint (default: http://localhost:9503)
- `--ws`: WebSocket endpoint (default: ws://localhost:9501/wss)

### Database Cleanup

Clean up test data after running tests:

```bash
./stress-test cleanup --config ./config.yaml
```

Options:
- `--config`: Path to config file (default: ./config.yaml)

**WARNING**: This will delete all test users and their related data. You'll be prompted to confirm before deletion.

## Test Statistics

After each test, the tool displays comprehensive statistics:

- Total Requests
- Successful Requests
- Failed Requests
- Total Connections
- Active Connections
- Total Messages Sent
- Test Duration
- Requests/second
- Messages/second

## Example Output

```
========== Group Chat Test ==========
Creating 10 groups with 20 members each, sending 100 messages per member...
Creating users...
Created 200 users successfully
Creating groups...
Created 10 groups successfully
Sending messages to groups...

========== Test Statistics ==========
Total Requests:       210
Successful Requests:  210
Failed Requests:      0
Total Connections:    0
Active Connections:   0
Total Messages Sent:  20000
Test Duration:        45.23 seconds
Requests/second:      4.64
Messages/second:      442.21
=====================================
```

## Architecture

The stress test tool consists of several components:

1. **main.go**: CLI interface and test orchestration
2. **http_client.go**: HTTP API client for REST operations
3. **websocket_client.go**: WebSocket client for real-time messaging
4. **cleanup.go**: Database cleanup utility

## Test User Naming Convention

All test users are created with a `stress_` prefix in their mobile/nickname field:
- `stress_user_*`: User creation test users
- `stress_friend_*`: Friend request test users
- `stress_group_*`: Group chat test users
- `stress_mixed_*`: Mixed operations test users

This allows for easy identification and cleanup of test data.

## Tips for Running Tests

1. **Start Small**: Begin with smaller test parameters to ensure everything works
2. **Monitor Resources**: Watch server CPU, memory, and database connections
3. **Check Logs**: Review server logs for errors or warnings
4. **Clean Up**: Always run the cleanup command after testing
5. **Incremental Testing**: Gradually increase test parameters to find system limits

## Troubleshooting

### Connection Refused

- Ensure the HTTP and WebSocket servers are running
- Verify the endpoints are correct
- Check firewall settings

### High Failure Rate

- The server may be overloaded
- Reduce concurrency or test parameters
- Check server logs for specific errors

### Out of Memory

- The test parameters may be too aggressive
- Reduce the number of concurrent users or messages
- Increase server resources

## Requirements

- Go 1.22 or higher
- Access to the IM system HTTP API
- Access to the IM system WebSocket server
- MySQL database connection (for cleanup)

## Development

To modify or extend the stress test tool:

1. Add new test scenarios in `main.go`
2. Extend HTTP client operations in `http_client.go`
3. Add WebSocket message types in `websocket_client.go`
4. Enhance cleanup logic in `cleanup.go`

## License

This tool is part of the IM system project and follows the same license.
