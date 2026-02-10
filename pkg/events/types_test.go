package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewBaseEvent(t *testing.T) {
	eventType := "test.event"
	region := "us-west-2"
	payload := map[string]interface{}{
		"key": "value",
	}

	event := NewBaseEvent(eventType, region, payload)

	if event.EventType != eventType {
		t.Errorf("Expected event type %s, got %s", eventType, event.EventType)
	}

	if event.SourceRegion != region {
		t.Errorf("Expected region %s, got %s", region, event.SourceRegion)
	}

	if event.EventID == "" {
		t.Error("EventID should not be empty")
	}

	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if event.Metadata.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", event.Metadata.Version)
	}
}

func TestBaseEvent_ToJSON(t *testing.T) {
	event := NewBaseEvent("test.event", "us-west-2", map[string]interface{}{
		"test": "data",
	})

	jsonData, err := event.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize event: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON data should not be empty")
	}

	// Verify it's valid JSON
	var decoded BaseEvent
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to deserialize JSON: %v", err)
	}

	if decoded.EventType != event.EventType {
		t.Errorf("Deserialized event type doesn't match")
	}
}

func TestFromJSON(t *testing.T) {
	original := NewBaseEvent("test.event", "us-west-2", map[string]interface{}{
		"test": "data",
	})

	jsonData, _ := original.ToJSON()
	decoded, err := FromJSON(jsonData)

	if err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if decoded.EventType != original.EventType {
		t.Errorf("Expected event type %s, got %s", original.EventType, decoded.EventType)
	}

	if decoded.SourceRegion != original.SourceRegion {
		t.Errorf("Expected region %s, got %s", original.SourceRegion, decoded.SourceRegion)
	}
}

func TestNewCDCEvent(t *testing.T) {
	operation := OperationInsert
	tableName := "customers"
	after := map[string]interface{}{
		"id":   "123",
		"name": "Test",
	}
	before := map[string]interface{}{}

	event := NewCDCEvent(operation, tableName, after, before)

	if event.Operation != operation {
		t.Errorf("Expected operation %s, got %s", operation, event.Operation)
	}

	if event.TableName != tableName {
		t.Errorf("Expected table %s, got %s", tableName, event.TableName)
	}

	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if event.After == nil {
		t.Error("After should not be nil")
	}
}

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"CustomerCreated", EventTypeCustomerCreated, "customer.created"},
		{"CustomerUpdated", EventTypeCustomerUpdated, "customer.updated"},
		{"CustomerDeleted", EventTypeCustomerDeleted, "customer.deleted"},
		{"CrossRegion", EventTypeCrossRegion, "cross_region.event"},
		{"HealthCheck", EventTypeHealthCheck, "health.check"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.constant)
			}
		})
	}
}

func TestOperationConstants(t *testing.T) {
	operations := []string{
		OperationInsert,
		OperationUpdate,
		OperationDelete,
		OperationRefresh,
	}

	expectedOps := []string{"INSERT", "UPDATE", "DELETE", "REFRESH"}

	for i, op := range operations {
		if op != expectedOps[i] {
			t.Errorf("Expected operation %s, got %s", expectedOps[i], op)
		}
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusHealthy != "healthy" {
		t.Errorf("Expected 'healthy', got %s", StatusHealthy)
	}

	if StatusDegraded != "degraded" {
		t.Errorf("Expected 'degraded', got %s", StatusDegraded)
	}

	if StatusUnhealthy != "unhealthy" {
		t.Errorf("Expected 'unhealthy', got %s", StatusUnhealthy)
	}
}

func TestCircuitBreakerConstants(t *testing.T) {
	if CircuitBreakerClosed != "closed" {
		t.Errorf("Expected 'closed', got %s", CircuitBreakerClosed)
	}

	if CircuitBreakerOpen != "open" {
		t.Errorf("Expected 'open', got %s", CircuitBreakerOpen)
	}

	if CircuitBreakerHalfOpen != "half_open" {
		t.Errorf("Expected 'half_open', got %s", CircuitBreakerHalfOpen)
	}
}

func TestCrossRegionEvent(t *testing.T) {
	base := NewBaseEvent("test.event", "us-west-2", map[string]interface{}{
		"data": "test",
	})

	crossRegion := &CrossRegionEvent{
		BaseEvent:         *base,
		TargetRegion:      "us-east-1",
		OriginalTimestamp: time.Now(),
		CompressionType:   "zstd",
		Checksum:          "abc123",
	}

	if crossRegion.TargetRegion != "us-east-1" {
		t.Errorf("Expected target region us-east-1, got %s", crossRegion.TargetRegion)
	}

	if crossRegion.CompressionType != "zstd" {
		t.Errorf("Expected compression type zstd, got %s", crossRegion.CompressionType)
	}
}

func TestDeadLetterEvent(t *testing.T) {
	originalEvent, _ := json.Marshal(map[string]string{"test": "data"})

	dlq := &DeadLetterEvent{
		OriginalEvent: originalEvent,
		ErrorMessage:  "Processing failed",
		ErrorType:     "validation_error",
		FailureCount:  3,
		FirstFailure:  time.Now().Add(-1 * time.Hour),
		LastFailure:   time.Now(),
		SourceHandler: "stream-processor",
	}

	if dlq.ErrorMessage != "Processing failed" {
		t.Errorf("Expected error message 'Processing failed', got %s", dlq.ErrorMessage)
	}

	if dlq.FailureCount != 3 {
		t.Errorf("Expected failure count 3, got %d", dlq.FailureCount)
	}

	if dlq.SourceHandler != "stream-processor" {
		t.Errorf("Expected source handler 'stream-processor', got %s", dlq.SourceHandler)
	}
}

func TestHealthCheckEvent(t *testing.T) {
	health := &HealthCheckEvent{
		Region:  "us-west-2",
		Service: "multi-region-eda",
		Status:  StatusHealthy,
		Timestamp: time.Now(),
		Dependencies: []DependencyCheck{
			{
				Name:      "dynamodb",
				Type:      "database",
				Status:    StatusHealthy,
				Latency:   50 * time.Millisecond,
				ErrorRate: 0.0,
			},
		},
		Metrics: HealthMetrics{
			CPUUsage:    45.5,
			MemoryUsage: 60.2,
			Latency:     50,
			Throughput:  1000,
			ErrorRate:   0.01,
		},
	}

	if health.Status != StatusHealthy {
		t.Errorf("Expected status %s, got %s", StatusHealthy, health.Status)
	}

	if len(health.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(health.Dependencies))
	}

	if health.Dependencies[0].Name != "dynamodb" {
		t.Errorf("Expected dependency name 'dynamodb', got %s", health.Dependencies[0].Name)
	}
}

func TestTransformedEvent(t *testing.T) {
	base := NewBaseEvent("test.event", "us-west-2", map[string]interface{}{
		"email": "test@example.com",
	})

	transformed := &TransformedEvent{
		BaseEvent:           *base,
		TransformationRules: []string{"validate", "enrich", "normalize"},
		EnrichmentData: map[string]interface{}{
			"geolocation": "Oregon",
		},
		ValidationErrors: []ValidationError{
			{
				Field:   "email",
				Message: "invalid format",
				Code:    "INVALID_FORMAT",
			},
		},
		TransformedAt: time.Now(),
	}

	if len(transformed.TransformationRules) != 3 {
		t.Errorf("Expected 3 transformation rules, got %d", len(transformed.TransformationRules))
	}

	if len(transformed.ValidationErrors) != 1 {
		t.Errorf("Expected 1 validation error, got %d", len(transformed.ValidationErrors))
	}

	if transformed.ValidationErrors[0].Field != "email" {
		t.Errorf("Expected field 'email', got %s", transformed.ValidationErrors[0].Field)
	}
}
