# Stress Test Tool - Complete Usage Guide

## Overview

The stress test tool is a comprehensive testing utility designed to simulate real-world usage patterns for the IM (Instant Messaging) system. It can test various operations including user creation, friend requests, group creation, and message sending at scale.

## Building the Tool

### Using Make (Recommended)

```bash
cd backend
make stress-test
```

### Using Go Build

```bash
cd backend/cmd/stress-test
go build -o stress-test
```

### Using the Quickstart Script

```bash
cd backend/cmd/stress-test
./quickstart.sh
```

## Command Structure

The tool uses a hierarchical command structure:

```
stress-test
├── interactive          # Interactive mode with prompts
├── scenario            # Predefined test scenarios
│   ├── user-creation   # Test user registration at scale
│   ├── friend-requests # Test friend request operations
│   ├── group-chat      # Test group messaging
│   └── mixed-operations # Combined operations test
└── cleanup             # Database cleanup utility
```

## Test Scenarios

### 1. User Creation Test

Simulates concurrent user registration to test the authentication system's scalability.

**What it does:**
- Creates specified number of users concurrently
- Generates unique usernames with timestamp prefix
- Uses consistent password for all test users
- Tracks success/failure rates

**Command:**
```bash
./stress-test scenario user-creation --count 100 --http http://localhost:9503
```

**Parameters:**
- `--count`: Number of users to create (default: 100)
- `--http`: HTTP API endpoint (default: http://localhost:9503)

**Example output:**
```
========== User Creation Test ==========
Creating 100 users...

========== Test Statistics ==========
Total Requests:       100
Successful Requests:  98
Failed Requests:      2
Test Duration:        12.34 seconds
Requests/second:      8.11
=====================================
```

### 2. Friend Request Test

Tests the friend request system by creating users and having them send friend requests to each other.

**What it does:**
- Creates specified number of users
- Each user sends friend requests to other users
- Tests concurrent friend request handling
- Validates request delivery

**Command:**
```bash
./stress-test scenario friend-requests \
  --users 50 \
  --requests 10 \
  --http http://localhost:9503
```

**Parameters:**
- `--users`: Number of users to create (default: 50)
- `--requests`: Friend requests per user (default: 10)
- `--http`: HTTP API endpoint (default: http://localhost:9503)

**Example output:**
```
========== Friend Request Test ==========
Creating 50 users and sending 10 friend requests each...
Creating users...
Created 50 users successfully
Sending friend requests...

========== Test Statistics ==========
Total Requests:       550
Successful Requests:  548
Failed Requests:      2
Test Duration:        25.67 seconds
Requests/second:      21.43
=====================================
```

### 3. Group Chat Test

Simulates group chat scenarios with multiple groups and concurrent message sending.

**What it does:**
- Creates users and organizes them into groups
- Each group has specified number of members
- All members send messages concurrently
- Tests group message delivery and scalability

**Command:**
```bash
./stress-test scenario group-chat \
  --groups 10 \
  --members 20 \
  --messages 100 \
  --http http://localhost:9503 \
  --ws ws://localhost:9501/wss
```

**Parameters:**
- `--groups`: Number of groups to create (default: 10)
- `--members`: Members per group (default: 20)
- `--messages`: Messages per member (default: 100)
- `--http`: HTTP API endpoint (default: http://localhost:9503)
- `--ws`: WebSocket endpoint (default: ws://localhost:9501/wss)

**Example output:**
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
Total Messages Sent:  20000
Test Duration:        45.23 seconds
Requests/second:      4.64
Messages/second:      442.21
=====================================
```

### 4. Mixed Operations Test

Runs a combination of different operations to simulate real-world usage patterns.

**What it does:**
- Creates initial set of users
- Runs for specified duration
- Randomly performs operations:
  - Send friend requests
  - Send private messages
  - Retrieve friend lists
  - Retrieve session lists
- Each user operates independently

**Command:**
```bash
./stress-test scenario mixed-operations \
  --duration 60 \
  --concurrency 50 \
  --http http://localhost:9503 \
  --ws ws://localhost:9501/wss
```

**Parameters:**
- `--duration`: Test duration in seconds (default: 60)
- `--concurrency`: Number of concurrent users (default: 50)
- `--http`: HTTP API endpoint (default: http://localhost:9503)
- `--ws`: WebSocket endpoint (default: ws://localhost:9501/wss)

**Example output:**
```
========== Mixed Operations Test ==========
Running for 60 seconds with 50 concurrent users...
Creating initial users...
Created 50 users successfully
Running mixed operations...

========== Test Statistics ==========
Total Requests:       15234
Successful Requests:  15180
Failed Requests:      54
Total Messages Sent:  3805
Active Connections:   0
Test Duration:        60.12 seconds
Requests/second:      253.48
Messages/second:      63.29
=====================================
```

## Interactive Mode

For users who prefer a guided experience:

```bash
./stress-test interactive
```

**Interactive flow:**
1. Enter HTTP endpoint (or press Enter for default)
2. Enter WebSocket endpoint (or press Enter for default)
3. Select test scenario from menu
4. Configure test parameters
5. Test runs automatically
6. View results

## Database Cleanup

After running tests, clean up test data:

```bash
./stress-test cleanup --config ../../config.yaml
```

**What it does:**
- Identifies all test users (prefix: `stress_*`)
- Deletes user messages
- Deletes contact applications
- Deletes contacts
- Deletes user sessions
- Deletes groups created by test users
- Deletes group messages and memberships
- Deletes test user accounts

**Safety features:**
- Requires confirmation before deletion
- Uses database transactions
- Rolls back on errors

**Warning:** This operation is irreversible. Always confirm you're cleaning test data only.

## Advanced Usage

### Custom Concurrency Limits

The tool uses a semaphore pattern to limit concurrent requests. Default is 50 concurrent operations. To modify, edit the source code:

```go
semaphore := make(chan struct{}, 100) // Change 50 to 100
```

### Custom Test Users

Test users are created with predictable patterns:
- User creation test: `stress_user_{timestamp}_{index}`
- Friend request test: `stress_friend_{timestamp}_{index}`
- Group chat test: `stress_group_{timestamp}_{index}`
- Mixed operations: `stress_mixed_{timestamp}_{index}`

### Monitoring Server Performance

While running tests, monitor:
- Server CPU usage
- Memory consumption
- Database connections
- Redis connections
- Network throughput
- Response times in logs

## Troubleshooting

### Connection Refused

**Symptom:** Tests fail with "connection refused" errors

**Solutions:**
1. Verify HTTP server is running on specified port
2. Verify WebSocket server is running
3. Check firewall settings
4. Confirm endpoints are correct

### High Failure Rate

**Symptom:** Many requests fail or timeout

**Solutions:**
1. Reduce concurrency parameters
2. Increase server resources
3. Check server logs for specific errors
4. Verify database can handle the load

### Database Errors

**Symptom:** Database connection or query errors

**Solutions:**
1. Check database connection settings
2. Verify database has enough connections
3. Check for table locks
4. Ensure database has sufficient resources

### Out of Memory

**Symptom:** Server or tool crashes with OOM errors

**Solutions:**
1. Reduce test parameters
2. Increase server memory
3. Run tests in smaller batches
4. Check for memory leaks in server code

## Performance Tuning

### Server Side

Before running large-scale tests:
1. Increase database connection pool size
2. Increase Redis connection pool size
3. Adjust HTTP server timeouts
4. Configure WebSocket limits
5. Enable server-side caching

### Client Side (Stress Test Tool)

For better performance:
1. Run tool on same network as server
2. Use multiple test instances for very large tests
3. Adjust concurrency based on server capacity
4. Use separate machines for different scenarios

## Best Practices

1. **Start Small:** Begin with small parameters to verify setup
2. **Incremental Testing:** Gradually increase load to find limits
3. **Monitor Resources:** Watch server metrics during tests
4. **Clean Up Regularly:** Run cleanup after each test session
5. **Document Results:** Record test parameters and results
6. **Baseline Testing:** Establish performance baselines
7. **Regression Testing:** Regular tests to detect performance degradation

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
name: Stress Test

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  stress-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Start Services
        run: docker-compose up -d
      
      - name: Wait for Services
        run: sleep 30
      
      - name: Run Stress Test
        run: |
          cd backend/cmd/stress-test
          go build -o stress-test
          ./stress-test scenario user-creation --count 100
      
      - name: Cleanup
        run: |
          cd backend/cmd/stress-test
          ./stress-test cleanup --config ../../config.yaml
```

## Interpreting Results

### Success Rate
- **95%+:** Excellent
- **90-95%:** Good, investigate failures
- **<90%:** Issues need attention

### Requests/Second
- Compare against baseline
- Higher is better
- Should scale linearly with resources

### Messages/Second
- Key metric for messaging system
- Watch for degradation under load
- Should remain consistent

### Test Duration
- Track over time
- Increasing duration indicates problems
- Should be consistent for same parameters

## Support and Contribution

For issues or improvements:
1. Check existing documentation
2. Review error logs
3. Create detailed bug reports
4. Submit pull requests for enhancements

## License

Part of the IM system project.
