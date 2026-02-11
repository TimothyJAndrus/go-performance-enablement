package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	wguevents "github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	// Initialize logger for tests
	logger, _ = zap.NewDevelopment()
	currentRegion = "us-west-2"
	partnerRegion = "us-east-1"
	eventBusName = "test-event-bus"
	dlqURL = "https://sqs.us-west-2.amazonaws.com/123456789012/test-dlq"
}

func TestNewCircuitBreaker(t *testing.T) {
	tests := []struct {
		name        string
		maxFailures int
		timeout     time.Duration
	}{
		{
			name:        "standard config",
			maxFailures: 5,
			timeout:     30 * time.Second,
		},
		{
			name:        "aggressive config",
			maxFailures: 3,
			timeout:     10 * time.Second,
		},
		{
			name:        "lenient config",
			maxFailures: 10,
			timeout:     60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker(tt.maxFailures, tt.timeout)
			
			assert.NotNil(t, cb)
			assert.Equal(t, tt.maxFailures, cb.maxFailures)
			assert.Equal(t, tt.timeout, cb.timeout)
			assert.Equal(t, wguevents.CircuitBreakerClosed, cb.state)
			assert.Equal(t, 0, cb.failureCount)
			assert.Equal(t, 0, cb.successCount)
		})
	}
}

func TestCircuitBreaker_GetState(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	
	state := cb.GetState()
	assert.Equal(t, wguevents.CircuitBreakerClosed, state)
}

func TestCircuitBreaker_Execute_SuccessPath(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	
	// Test successful execution
	err := cb.Execute(func() error {
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
	assert.Equal(t, 0, cb.failureCount)
	assert.Equal(t, 1, cb.successCount)
}

func TestCircuitBreaker_Execute_FailureAccumulation(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)
	
	// Cause failures but not enough to open circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(func() error {
			return assert.AnError
		})
		assert.Error(t, err)
		assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
	}
	
	assert.Equal(t, 2, cb.failureCount)
	
	// One more failure should open the circuit
	err := cb.Execute(func() error {
		return assert.AnError
	})
	assert.Error(t, err)
	assert.Equal(t, wguevents.CircuitBreakerOpen, cb.GetState())
	assert.Equal(t, 3, cb.failureCount)
}

func TestCircuitBreaker_Execute_OpenCircuit(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return assert.AnError
		})
	}
	
	assert.Equal(t, wguevents.CircuitBreakerOpen, cb.GetState())
	
	// Circuit should reject calls when open
	err := cb.Execute(func() error {
		t.Error("Function should not be called when circuit is open")
		return nil
	})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreaker_Execute_HalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return assert.AnError
		})
	}
	
	assert.Equal(t, wguevents.CircuitBreakerOpen, cb.GetState())
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Next call should transition to half-open
	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "Function should be called once in half-open state")
	assert.Equal(t, wguevents.CircuitBreakerHalfOpen, cb.GetState())
}

func TestCircuitBreaker_Execute_HalfOpenToClosedTransition(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return assert.AnError
		})
	}
	
	// Wait for timeout and transition to half-open
	time.Sleep(100 * time.Millisecond)
	
	// First successful call in half-open
	err := cb.Execute(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, wguevents.CircuitBreakerHalfOpen, cb.GetState())
	
	// Second successful call should close the circuit
	err = cb.Execute(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
	assert.Equal(t, 0, cb.failureCount)
}

func TestCircuitBreaker_Execute_HalfOpenToOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return assert.AnError
		})
	}
	
	// Wait for timeout and transition to half-open
	time.Sleep(100 * time.Millisecond)
	
	// Failure in half-open should immediately open circuit
	err := cb.Execute(func() error {
		return assert.AnError
	})
	
	assert.Error(t, err)
	assert.Equal(t, wguevents.CircuitBreakerOpen, cb.GetState())
}

func TestCircuitBreaker_Concurrency(t *testing.T) {
	cb := NewCircuitBreaker(10, 30*time.Second)
	
	// Test that circuit breaker is thread-safe
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func() {
			_ = cb.Execute(func() error {
				time.Sleep(1 * time.Millisecond)
				return nil
			})
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
	assert.Equal(t, 10, cb.successCount)
}

func TestParseRecord(t *testing.T) {
	tests := []struct {
		name      string
		record    events.DynamoDBEventRecord
		expectErr bool
	}{
		{
			name: "valid record with new image",
			record: events.DynamoDBEventRecord{
				EventID:   "event-123",
				EventName: "INSERT",
				Change: events.DynamoDBStreamRecord{
					NewImage: map[string]events.DynamoDBAttributeValue{
						"id": events.NewStringAttribute("123"),
						"name": events.NewStringAttribute("test"),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "empty record",
			record: events.DynamoDBEventRecord{
				EventID:   "event-456",
				EventName: "REMOVE",
				Change: events.DynamoDBStreamRecord{
					NewImage: map[string]events.DynamoDBAttributeValue{},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := parseRecord(tt.record)
			
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, event)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, event)
				assert.Equal(t, tt.record.EventID, event.EventID)
				assert.Equal(t, "dynamodb-streams", event.Metadata.SourceService)
			}
		})
	}
}

func TestCompressEvent(t *testing.T) {
	event := &wguevents.CrossRegionEvent{
		BaseEvent: wguevents.BaseEvent{
			EventID:   "test-123",
			EventType: "test.event",
			Payload: map[string]interface{}{
				"data": "This is test data that should compress well",
			},
		},
		TargetRegion:      "us-east-1",
		OriginalTimestamp: time.Now(),
		CompressionType:   "zstd",
	}

	compressed, err := compressEvent(event)
	
	assert.NoError(t, err)
	assert.NotNil(t, compressed)
	
	// Verify compression worked (compressed should be smaller or similar size for small data)
	originalJSON, _ := json.Marshal(event)
	t.Logf("Original size: %d, Compressed size: %d", len(originalJSON), len(compressed))
	
	// Compressed data should be non-empty
	assert.Greater(t, len(compressed), 0)
}

func TestCompressEvent_EmptyEvent(t *testing.T) {
	event := &wguevents.CrossRegionEvent{
		BaseEvent: wguevents.BaseEvent{
			EventID:   "empty-event",
			EventType: "empty",
			Payload:   map[string]interface{}{},
		},
		TargetRegion: "us-east-1",
	}

	compressed, err := compressEvent(event)
	
	assert.NoError(t, err)
	assert.NotNil(t, compressed)
	assert.Greater(t, len(compressed), 0)
}

func TestSendToDLQ_EventCreation(t *testing.T) {
	event := &wguevents.BaseEvent{
		EventID:   "test-event-123",
		EventType: "test.event",
		Payload: map[string]interface{}{
			"test": "data",
		},
	}

	processingError := assert.AnError

	// Create DLQ event (similar to sendToDLQ logic)
	dlqEvent := &wguevents.DeadLetterEvent{
		ErrorMessage:  processingError.Error(),
		ErrorType:     "routing_failure",
		FailureCount:  1,
		FirstFailure:  time.Now(),
		LastFailure:   time.Now(),
		SourceHandler: "event-router",
	}

	originalJSON, err := json.Marshal(event)
	assert.NoError(t, err)
	dlqEvent.OriginalEvent = originalJSON

	messageBody, err := json.Marshal(dlqEvent)
	assert.NoError(t, err)
	assert.NotEmpty(t, messageBody)

	// Verify DLQ event structure
	var parsedDLQ wguevents.DeadLetterEvent
	err = json.Unmarshal(messageBody, &parsedDLQ)
	assert.NoError(t, err)
	assert.Equal(t, "routing_failure", parsedDLQ.ErrorType)
	assert.Equal(t, "event-router", parsedDLQ.SourceHandler)
	assert.Equal(t, 1, parsedDLQ.FailureCount)
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	// Test complete state machine: Closed -> Open -> Half-Open -> Closed
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	
	// Initial state: Closed
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
	
	// Cause failures to open circuit
	_ = cb.Execute(func() error { return assert.AnError })
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState()) // Still closed after 1 failure
	
	_ = cb.Execute(func() error { return assert.AnError })
	assert.Equal(t, wguevents.CircuitBreakerOpen, cb.GetState()) // Now open after 2 failures
	
	// Try to execute while open (should fail immediately)
	err := cb.Execute(func() error {
		t.Error("Should not execute when circuit is open")
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, wguevents.CircuitBreakerOpen, cb.GetState())
	
	// Wait for timeout
	time.Sleep(100 * time.Millisecond)
	
	// Execute should transition to half-open
	_ = cb.Execute(func() error { return nil })
	assert.Equal(t, wguevents.CircuitBreakerHalfOpen, cb.GetState())
	
	// Second success should close circuit
	_ = cb.Execute(func() error { return nil })
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
}

func TestCircuitBreaker_ResetOnClose(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	
	// Open circuit
	_ = cb.Execute(func() error { return assert.AnError })
	_ = cb.Execute(func() error { return assert.AnError })
	assert.Equal(t, 2, cb.failureCount)
	
	// Transition to half-open and then closed
	time.Sleep(100 * time.Millisecond)
	_ = cb.Execute(func() error { return nil })
	_ = cb.Execute(func() error { return nil })
	
	assert.Equal(t, wguevents.CircuitBreakerClosed, cb.GetState())
	assert.Equal(t, 0, cb.failureCount) // Should be reset
}

func TestParseRecord_MetadataPopulation(t *testing.T) {
	record := events.DynamoDBEventRecord{
		EventID:   "test-event-id",
		EventName: "MODIFY",
		Change: events.DynamoDBStreamRecord{
			NewImage: map[string]events.DynamoDBAttributeValue{
				"userId": events.NewStringAttribute("user-123"),
				"action": events.NewStringAttribute("update"),
			},
		},
	}

	event, err := parseRecord(record)
	
	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, "test-event-id", event.EventID)
	assert.Equal(t, "MODIFY", event.EventType)
	assert.Equal(t, currentRegion, event.SourceRegion)
	assert.Equal(t, "dynamodb-streams", event.Metadata.SourceService)
	assert.NotEmpty(t, event.Payload)
}
