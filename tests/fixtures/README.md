# Test Fixtures

This directory contains sample JSON files used for testing the Go Performance Enablement project.

## Fixture Files

### Event Types

| File | Description |
|------|-------------|
| `base_event.json` | Standard base event structure used across all event types |
| `cross_region_event.json` | Cross-region replication event with compression metadata |
| `transformed_event.json` | Event after validation and enrichment processing |
| `health_check_event.json` | Multi-region health status aggregation event |
| `dead_letter_event.json` | Failed event with error details and retry history |

### CDC Events

| File | Description |
|------|-------------|
| `cdc_event_insert.json` | INSERT operation from Qlik CDC |
| `cdc_event_update.json` | UPDATE operation with before/after states |
| `cdc_event_delete.json` | DELETE operation with before state |
| `kafka_cdc_message.json` | Full Kafka message structure with CDC payload |

### AWS Integration

| File | Description |
|------|-------------|
| `dynamodb_stream_record.json` | DynamoDB Streams event with multiple records |
| `authorizer_event.json` | API Gateway Lambda authorizer request |

## Usage

### Loading Fixtures in Tests

```go
package mypackage_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func loadFixture(t *testing.T, filename string) []byte {
    t.Helper()
    path := filepath.Join("../../tests/fixtures", filename)
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("Failed to load fixture %s: %v", filename, err)
    }
    return data
}

func TestWithFixture(t *testing.T) {
    data := loadFixture(t, "base_event.json")
    
    var event BaseEvent
    if err := json.Unmarshal(data, &event); err != nil {
        t.Fatalf("Failed to parse fixture: %v", err)
    }
    
    // Test assertions...
}
```

### Fixture Helper Function

A helper function is available in `pkg/testutil/fixtures.go` (if created):

```go
import "github.com/wgu/go-performance-enablement/pkg/testutil"

func TestExample(t *testing.T) {
    event := testutil.LoadBaseEvent(t)
    // ...
}
```

## Conventions

1. **Timestamps**: Use ISO 8601 format (`2026-02-11T10:30:00Z`)
2. **IDs**: Use descriptive prefixes (`evt-`, `cust-`, `corr-`)
3. **Regions**: Use `us-west-2` as primary, `us-east-1` as secondary
4. **Sensitive Data**: Use placeholder values, never real credentials

## Updating Fixtures

When modifying event schemas:

1. Update the corresponding fixture file
2. Run tests to ensure compatibility
3. Update this README if adding new fixtures

## Related Documentation

- [Event Types](/pkg/events/types.go) - Event struct definitions
- [Testing Guide](/README.md#testing) - How to run tests
