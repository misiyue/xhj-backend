# Security Middleware Documentation

This document describes the security middleware implemented in the Lumen IM backend to prevent brute-force attacks, control QPS (Queries Per Second), and enhance overall security.

## Overview

The backend implements multiple layers of security middleware:

1. **Rate Limiting** - Controls request rates per IP and globally
2. **Brute-Force Protection** - Prevents password cracking attempts
3. **Security Headers** - Adds standard security HTTP headers
4. **Request Size Limits** - Prevents large payload attacks

## 1. Rate Limiting Middleware

Rate limiting controls the number of requests that can be made to the server, preventing abuse and ensuring fair resource allocation.

### Features

- **Per-IP Rate Limiting**: Each IP address has its own rate limit
- **Global Rate Limiting**: Overall system QPS control
- **Token Bucket Algorithm**: Allows burst traffic while maintaining average rate
- **Automatic Cleanup**: Removes stale rate limiters to prevent memory leaks

### Configuration

Add the following to your `config.yaml`:

```yaml
security:
  # Per-IP rate limiting
  rate_limit:
    enabled: true       # Enable/disable per-IP rate limiting
    capacity: 100       # Token bucket capacity (max burst requests)
    rate: 50           # Tokens generated per second (QPS per IP)
  
  # Global rate limiting
  global_rate_limit:
    enabled: true       # Enable/disable global rate limiting
    capacity: 1000      # Token bucket capacity (max burst requests)
    rate: 500          # Tokens generated per second (global QPS)
```

### How It Works

The rate limiter uses the **Token Bucket Algorithm**:

1. Each IP gets a bucket with a maximum capacity of tokens
2. Tokens are generated at a fixed rate (e.g., 50 tokens/second)
3. Each request consumes one token
4. If no tokens are available, the request is rejected with HTTP 429

**Example:**
- Capacity: 100, Rate: 50/sec
- Initial state: 100 tokens available
- Burst of 100 requests: All pass immediately
- 101st request: Rejected (no tokens)
- After 1 second: 50 new tokens added
- Next 50 requests: Pass

### Response Codes

- **200 OK**: Request allowed
- **429 Too Many Requests**: Rate limit exceeded (per-IP)
- **503 Service Unavailable**: Global rate limit exceeded

## 2. Brute-Force Protection Middleware

Protects sensitive endpoints (especially login) from brute-force attacks by tracking failed attempts and temporarily blocking offending IPs.

### Features

- **Attempt Tracking**: Monitors failed login attempts per IP
- **Progressive Blocking**: Uses exponential backoff for repeat offenders
- **Automatic Reset**: Clears failed attempts after a configurable period
- **Success Reset**: Clears counter on successful login

### Configuration

```yaml
security:
  brute_force:
    enabled: true       # Enable/disable brute-force protection
    max_attempts: 5     # Maximum failed attempts before blocking
    block_duration: 15  # Initial block duration in minutes
    reset_duration: 30  # Time to reset failure count in minutes
```

### How It Works

1. **Tracking Failures**: Each failed login attempt is recorded by IP
2. **Blocking**: After `max_attempts` failures, IP is blocked
3. **Exponential Backoff**: Each additional attempt increases block time
4. **Auto Reset**: If no attempts for `reset_duration`, counter resets

**Example:**
- Max attempts: 5
- Block duration: 15 minutes
- After 5 failed logins: Blocked for 15 minutes
- After 6 failed logins: Blocked for 30 minutes (2x)
- After 7 failed logins: Blocked for 45 minutes (3x)
- Maximum block time: 1 hour

### Integration

To integrate brute-force protection in your login handler:

```go
import "github.com/gzydong/go-chat/internal/pkg/core/middleware"

func LoginHandler(c *gin.Context) {
    // Your login logic here
    success := authenticateUser(username, password)
    
    // Record the attempt
    middleware.RecordLoginAttempt(c, success)
    
    if !success {
        c.JSON(401, gin.H{"message": "Invalid credentials"})
        return
    }
    
    // Continue with successful login
}
```

### Response Codes

- **200 OK**: Login attempt allowed
- **429 Too Many Requests**: IP is blocked due to too many failed attempts

## 3. Security Headers Middleware

Adds security-related HTTP headers to all responses to protect against common web vulnerabilities.

### Features

- **XSS Protection**: Prevents cross-site scripting attacks
- **Clickjacking Prevention**: Prevents UI redress attacks
- **MIME Sniffing Protection**: Prevents MIME confusion attacks
- **HSTS**: Forces HTTPS connections
- **CSP**: Content Security Policy for XSS prevention
- **Referrer Policy**: Controls referrer information leakage

### Configuration

```yaml
security:
  security_headers:
    enabled: true
    x_frame_options: "SAMEORIGIN"                      # Prevent clickjacking
    x_content_type_options: "nosniff"                  # Prevent MIME sniffing
    xss_protection: "1; mode=block"                    # Enable XSS filter
    strict_transport_security: "max-age=31536000; includeSubDomains"  # Force HTTPS
    content_security_policy: "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'"
    referrer_policy: "strict-origin-when-cross-origin" # Control referrer info
    permissions_policy: "geolocation=(), microphone=(), camera=()"  # Disable features
```

### Headers Applied

| Header | Purpose | Default Value |
|--------|---------|---------------|
| X-Frame-Options | Prevents clickjacking | SAMEORIGIN |
| X-Content-Type-Options | Prevents MIME sniffing | nosniff |
| X-XSS-Protection | Enables browser XSS filter | 1; mode=block |
| Strict-Transport-Security | Forces HTTPS | max-age=31536000 |
| Content-Security-Policy | Controls resource loading | default-src 'self' |
| Referrer-Policy | Controls referrer header | strict-origin-when-cross-origin |
| Permissions-Policy | Controls browser features | geolocation=(), microphone=() |

### Security Benefits

- **X-Frame-Options: SAMEORIGIN**: Prevents your site from being embedded in iframes on other domains
- **X-Content-Type-Options: nosniff**: Prevents browsers from MIME-sniffing responses
- **X-XSS-Protection**: Enables browser's built-in XSS protection
- **HSTS**: Ensures all connections use HTTPS for 1 year
- **CSP**: Restricts sources of content to prevent XSS attacks
- **Referrer-Policy**: Prevents leaking sensitive URLs to third parties

## 4. Request Size Limit Middleware

Prevents denial-of-service attacks by limiting the size of request bodies.

### Configuration

```yaml
security:
  request_size:
    enabled: true
    max_body_size: 10485760  # 10MB in bytes
```

### How It Works

- Limits incoming request body size
- Returns error if request exceeds limit
- Prevents memory exhaustion attacks
- Default limit: 10MB

### Response Codes

- **200 OK**: Request size is acceptable
- **413 Payload Too Large**: Request body exceeds limit

## Architecture

### Middleware Chain

The security middleware is applied in the following order:

1. **CORS** - Handle cross-origin requests
2. **Security Headers** - Add security headers
3. **Global Rate Limit** - System-wide QPS control
4. **Per-IP Rate Limit** - Individual IP rate limiting
5. **Request Size Limit** - Body size validation
6. **Brute-Force Protection** - (Applied to specific endpoints)
7. **Prometheus** - Metrics collection
8. **Access Log** - Request logging
9. **Authentication** - JWT validation

### Performance Considerations

- **Token Bucket Algorithm**: O(1) time complexity for rate checking
- **Memory Management**: Automatic cleanup of stale entries
- **Concurrent Safe**: Uses sync.Map for thread-safe operations
- **Minimal Overhead**: Lightweight middleware with negligible latency

### Monitoring

The middleware integrates with Prometheus for monitoring:

```
# Rate limiting metrics
http_requests_total{status="429"} - Rate limited requests
http_requests_total{status="503"} - Globally rate limited requests

# General metrics
http_request_duration_seconds - Request latency
http_requests_in_flight - Concurrent requests
```

## Testing

Comprehensive test suite included:

```bash
# Run all security middleware tests
go test ./internal/pkg/core/middleware -v

# Run specific tests
go test ./internal/pkg/core/middleware -run TestRateLimit
go test ./internal/pkg/core/middleware -run TestBruteForce
go test ./internal/pkg/core/middleware -run TestSecurityHeaders
```

Test coverage includes:
- Rate limiting with different configurations
- Brute-force protection scenarios
- Security header application
- Request size validation
- Edge cases and error conditions

## Best Practices

1. **Rate Limiting**
   - Set conservative limits initially
   - Monitor actual traffic patterns
   - Adjust based on legitimate use cases
   - Consider different limits for different endpoints

2. **Brute-Force Protection**
   - Apply to all authentication endpoints
   - Use appropriate max_attempts (3-5 recommended)
   - Monitor blocked IPs for patterns
   - Implement IP whitelist for trusted sources if needed

3. **Security Headers**
   - Keep CSP as restrictive as possible
   - Only add 'unsafe-inline' if absolutely necessary
   - Test thoroughly after changing CSP
   - Use HTTPS in production for HSTS to work

4. **Request Size Limits**
   - Set appropriate limits based on use case
   - Use larger limits for file upload endpoints
   - Monitor for legitimate large requests being blocked

## Troubleshooting

### Too Many 429 Errors

**Symptom**: Legitimate users getting rate limited

**Solutions**:
- Increase `capacity` for burst traffic
- Increase `rate` for higher sustained QPS
- Check for misconfigured load balancers (all requests from same IP)
- Implement authenticated user rate limiting separate from IP

### Brute-Force False Positives

**Symptom**: Legitimate users getting blocked

**Solutions**:
- Increase `max_attempts`
- Decrease `block_duration`
- Implement CAPTCHA after 2-3 failures
- Add IP whitelist for trusted networks

### CSP Errors in Browser Console

**Symptom**: Resources blocked by Content Security Policy

**Solutions**:
- Review browser console for specific violations
- Add necessary sources to CSP directives
- Use CSP report-only mode during testing
- Balance security with functionality

## Security Checklist

- [ ] Rate limiting enabled and configured
- [ ] Brute-force protection enabled for login endpoints
- [ ] Security headers properly configured
- [ ] Request size limits set appropriately
- [ ] HTTPS enforced in production (for HSTS)
- [ ] Monitoring and alerting configured
- [ ] Regular security audits performed
- [ ] Configuration reviewed and tested

## References

- [OWASP Rate Limiting](https://owasp.org/www-community/controls/Blocking_Brute_Force_Attacks)
- [OWASP Security Headers](https://owasp.org/www-project-secure-headers/)
- [Token Bucket Algorithm](https://en.wikipedia.org/wiki/Token_bucket)
- [Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP)
