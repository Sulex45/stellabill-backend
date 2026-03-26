# API Error Envelope Standardization

## Overview

This document describes the standardized error response envelope used across all API endpoints in the Stellabill backend. This ensures consistent error handling, improved observability, and better client error handling.

## Error Response Format

All error responses follow a standardized JSON envelope structure:

```json
{
  "code": "ERROR_CODE",
  "message": "Human-readable error message",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "details": {
    "field": "optional",
    "reason": "additional context"
  }
}
```

### Fields

- **code** (string, required): Machine-readable error code for programmatic error handling
  - Examples: `NOT_FOUND`, `UNAUTHORIZED`, `VALIDATION_FAILED`, `INTERNAL_ERROR`
- **message** (string, required): Human-readable error description
- **trace_id** (string, required): Unique identifier for this request, used for logging and debugging
  - Format: UUID v4
  - Persisted in response headers and logs for request tracking
- **details** (object, optional): Additional context-specific information
  - Used for validation errors to indicate which field failed and why

## Error Codes

### Client Errors (4xx)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request parameters or format |
| `VALIDATION_FAILED` | 400 | Input validation failed (detailed in `details`) |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication credentials |
| `FORBIDDEN` | 403 | Authenticated user lacks permission for resource |
| `NOT_FOUND` | 404 | Requested resource does not exist |
| `CONFLICT` | 409 | Request conflicts with current resource state |

### Server Errors (5xx)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

## Examples

### Not Found Error

```bash
$ curl -H "Authorization: Bearer <token>" \
       -H "X-Tenant-ID: tenant-1" \
       http://localhost:8080/api/subscriptions/nonexistent

HTTP/1.1 404 Not Found
X-Trace-ID: 550e8400-e29b-41d4-a716-446655440000

{
  "code": "NOT_FOUND",
  "message": "The requested resource was not found",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Validation Error

```bash
$ curl -H "Authorization: Bearer <token>" \
       -H "X-Tenant-ID: tenant-1" \
       http://localhost:8080/api/subscriptions/

HTTP/1.1 400 Bad Request
X-Trace-ID: 550e8400-e29b-41d4-a716-446655440000

{
  "code": "VALIDATION_FAILED",
  "message": "subscription id is required",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "details": {
    "field": "id",
    "reason": "cannot be empty"
  }
}
```

### Unauthorized Error

```bash
$ curl http://localhost:8080/api/subscriptions/sub-123

HTTP/1.1 401 Unauthorized
X-Trace-ID: 550e8400-e29b-41d4-a716-446655440000

{
  "code": "UNAUTHORIZED",
  "message": "authorization header required",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Trace ID Tracking

Every request is assigned a unique trace ID for request tracking and debugging:

1. If client provides `X-Trace-ID` header, that value is used
2. Otherwise, a new UUID is generated
3. Trace ID is available in:
   - Context (`c.GetString("traceID")`)
   - Response body (`error.trace_id`)
   - Response headers (`X-Trace-ID`)
   - Application logs (for integration with observability tools)

This allows correlating client requests with server logs and metrics.

## Implementation Details

### Error Mapping

Service layer errors are automatically mapped to HTTP status codes and error codes:

```go
// maps service.ErrNotFound → 404 NOT_FOUND
// maps service.ErrForbidden → 403 FORBIDDEN
// maps service.ErrDeleted → 410 Gone with NOT_FOUND code
// maps service.ErrBillingParse → 500 INTERNAL_ERROR
```

### Centralized Error Helpers

All error responses use helper functions in `internal/handlers/errors.go`:

```go
// Generic error response
RespondWithError(c, http.StatusNotFound, ErrorCodeNotFound, "Not found")

// Error with additional details
RespondWithErrorDetails(c, http.StatusBadRequest, ErrorCodeValidationFailed, 
  "Invalid input", map[string]interface{}{
    "field": "email",
    "reason": "invalid format",
  })

// Convenience helpers
RespondWithAuthError(c, "Missing auth header")
RespondWithNotFoundError(c, "subscription")
RespondWithValidationError(c, "Invalid input", details)
```

### Middleware Integration

The `TraceIDMiddleware` in `internal/middleware/traceid.go`:
- Injects trace ID into request context
- Sets `X-Trace-ID` response header
- Uses provided trace ID from client or generates a new UUID

Register middleware in routes:
```go
r.Use(middleware.TraceIDMiddleware())
```

## Security Considerations

1. **Error Messages**: Error messages are generic for security errors to avoid information disclosure
   - Bad authentication returns: "invalid or expired token" (not which part failed)
   - Permission denied returns: "forbidden" (not why)

2. **Trace IDs**: 
   - Used for audit logging and debugging
   - Never expose sensitive data in trace ID values
   - Trace IDs are UUIDs and don't contain information

3. **Details Field**:
   - Only use for validation errors with safe information
   - Never include passwords, tokens, or sensitive data
   - Example: `{"field": "email", "reason": "invalid format"}` ✅
   - Never: `{"field": "password", "received": "hunter2"}` ❌

## Testing Error Responses

Error handling is comprehensively tested in:
- `internal/handlers/errors_test.go` - Error envelope format and mapping
- `internal/handlers/subscriptions_test.go` - Integration with subscription handler
- `internal/middleware/traceid_test.go` - Trace ID generation and tracking

Test coverage includes:
- All error codes and HTTP status mappings
- Validation errors with details
- Authentication and authorization errors
- Trace ID generation and propagation
- Content-type headers
- Response envelope structure
