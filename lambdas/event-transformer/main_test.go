package main

import (
	"context"
	"strings"
	"testing"
	"time"

	wguevents "github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestNewEventValidator(t *testing.T) {
	validator := NewEventValidator()
	
	assert.NotNil(t, validator)
	assert.NotNil(t, validator.emailRegex)
	assert.NotNil(t, validator.uuidRegex)
}

func TestEventValidator_Validate_ValidEvent(t *testing.T) {
	validator := NewEventValidator()
	
	event := &wguevents.BaseEvent{
		EventID:      "test-event-123",
		EventType:    "user.created",
		SourceRegion: "us-west-2",
		Timestamp:    time.Now(),
		Metadata: wguevents.EventMetadata{
			SourceService: "user-service",
			TraceID:       "trace-123",
		},
		Payload: map[string]interface{}{
			"email": "test@example.com",
		},
	}
	
	errors := validator.Validate(event)
	assert.Empty(t, errors, "Valid event should have no validation errors")
}

func TestEventValidator_Validate_MissingRequiredFields(t *testing.T) {
	validator := NewEventValidator()
	
	event := &wguevents.BaseEvent{
		// Missing EventID, EventType, SourceRegion, Timestamp
		Metadata: wguevents.EventMetadata{
			// Missing SourceService, TraceID
		},
	}
	
	errors := validator.Validate(event)
	
	assert.NotEmpty(t, errors)
	
	// Check for specific required field errors
	errorFields := make(map[string]bool)
	for _, err := range errors {
		errorFields[err.Field] = true
	}
	
	assert.True(t, errorFields["event_id"], "Should have event_id error")
	assert.True(t, errorFields["event_type"], "Should have event_type error")
	assert.True(t, errorFields["source_region"], "Should have source_region error")
	assert.True(t, errorFields["timestamp"], "Should have timestamp error")
	assert.True(t, errorFields["metadata.source_service"], "Should have source_service error")
	assert.True(t, errorFields["metadata.trace_id"], "Should have trace_id error")
}

func TestEventValidator_Validate_FutureTimestamp(t *testing.T) {
	validator := NewEventValidator()
	
	futureTime := time.Now().Add(10 * time.Minute)
	event := &wguevents.BaseEvent{
		EventID:      "test-event-123",
		EventType:    "user.created",
		SourceRegion: "us-west-2",
		Timestamp:    futureTime,
		Metadata: wguevents.EventMetadata{
			SourceService: "user-service",
			TraceID:       "trace-123",
		},
	}
	
	errors := validator.Validate(event)
	
	assert.NotEmpty(t, errors)
	
	// Should have a timestamp validation error
	hasTimestampError := false
	for _, err := range errors {
		if err.Field == "timestamp" && err.Code == "INVALID_TIMESTAMP" {
			hasTimestampError = true
			break
		}
	}
	assert.True(t, hasTimestampError, "Should have timestamp validation error")
}

func TestEventValidator_Validate_InvalidEmail(t *testing.T) {
	validator := NewEventValidator()
	
	event := &wguevents.BaseEvent{
		EventID:      "test-event-123",
		EventType:    "user.created",
		SourceRegion: "us-west-2",
		Timestamp:    time.Now(),
		Metadata: wguevents.EventMetadata{
			SourceService: "user-service",
			TraceID:       "trace-123",
		},
		Payload: map[string]interface{}{
			"email": "invalid-email-format",
		},
	}
	
	errors := validator.Validate(event)
	
	assert.NotEmpty(t, errors)
	
	// Should have an email validation error
	hasEmailError := false
	for _, err := range errors {
		if err.Field == "payload.email" && err.Code == "INVALID_FORMAT" {
			hasEmailError = true
			break
		}
	}
	assert.True(t, hasEmailError, "Should have email validation error")
}

func TestEventValidator_Validate_ValidEmails(t *testing.T) {
	validator := NewEventValidator()
	
	validEmails := []string{
		"test@example.com",
		"user.name@domain.co.uk",
		"user+tag@example.org",
		"user_name123@test-domain.com",
	}
	
	for _, email := range validEmails {
		t.Run(email, func(t *testing.T) {
			event := &wguevents.BaseEvent{
				EventID:      "test-event-123",
				EventType:    "user.created",
				SourceRegion: "us-west-2",
				Timestamp:    time.Now(),
				Metadata: wguevents.EventMetadata{
					SourceService: "user-service",
					TraceID:       "trace-123",
				},
				Payload: map[string]interface{}{
					"email": email,
				},
			}
			
			errors := validator.Validate(event)
			
			// Should not have email validation error
			for _, err := range errors {
				assert.NotEqual(t, "payload.email", err.Field, "Should not have email validation error for valid email")
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with spaces",
			input:    "test @example.com",
			expected: "test@example.com",
		},
		{
			name:     "with multiple spaces",
			input:    "test  user @ example.com",
			expected: "testuser@example.com",
		},
		{
			name:     "with leading/trailing spaces",
			input:    " test@example.com ",
			expected: "test@example.com",
		},
		{
			name:     "normal email",
			input:    "test@example.com",
			expected: "test@example.com",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEmail(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with dashes",
			input:    "123-456-7890",
			expected: "1234567890",
		},
		{
			name:     "with spaces",
			input:    "123 456 7890",
			expected: "1234567890",
		},
		{
			name:     "with parentheses",
			input:    "(123) 456-7890",
			expected: "1234567890",
		},
		{
			name:     "with plus prefix",
			input:    "+1-123-456-7890",
			expected: "+11234567890",
		},
		{
			name:     "clean number",
			input:    "1234567890",
			expected: "1234567890",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePhone(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTimezoneForRegion(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us-west-2", "America/Los_Angeles"},
		{"us-east-1", "America/New_York"},
		{"eu-west-1", "Europe/Dublin"},
		{"ap-southeast-1", "Asia/Singapore"},
		{"unknown-region", "UTC"},
		{"", "UTC"},
	}
	
	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			result := getTimezoneForRegion(tt.region)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDataCenterForRegion(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us-west-2", "Oregon"},
		{"us-east-1", "Virginia"},
		{"eu-west-1", "Ireland"},
		{"ap-southeast-1", "Singapore"},
		{"unknown-region", "Unknown"},
		{"", "Unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			result := getDataCenterForRegion(tt.region)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnrichEvent(t *testing.T) {
	ctx := context.Background()
	
	event := &wguevents.TransformedEvent{
		BaseEvent: wguevents.BaseEvent{
			SourceRegion: "us-west-2",
		},
	}
	
	err := enrichEvent(ctx, event)
	
	assert.NoError(t, err)
	assert.NotNil(t, event.EnrichmentData)
	
	// Check region metadata
	regionMetadata, ok := event.EnrichmentData["region_metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "us-west-2", regionMetadata["region"])
	assert.Equal(t, "America/Los_Angeles", regionMetadata["timezone"])
	assert.Equal(t, "Oregon", regionMetadata["data_center"])
	
	// Check processing metadata
	processingMetadata, ok := event.EnrichmentData["processing_metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "event-transformer", processingMetadata["processor"])
	assert.Equal(t, "1.0.0", processingMetadata["version"])
	assert.NotNil(t, processingMetadata["processed_at"])
}

func TestNormalizeEvent_Email(t *testing.T) {
	event := &wguevents.TransformedEvent{
		BaseEvent: wguevents.BaseEvent{
			Payload: map[string]interface{}{
				"email": " Test@ Example.com ",
			},
		},
	}
	
	normalizeEvent(event)
	
	email, ok := event.Payload["email"].(string)
	assert.True(t, ok)
	// Email should be trimmed of spaces
	assert.False(t, strings.Contains(email, " "))
}

func TestNormalizeEvent_Phone(t *testing.T) {
	event := &wguevents.TransformedEvent{
		BaseEvent: wguevents.BaseEvent{
			Payload: map[string]interface{}{
				"phone": "(123) 456-7890",
			},
		},
	}
	
	normalizeEvent(event)
	
	phone, ok := event.Payload["phone"].(string)
	assert.True(t, ok)
	assert.Equal(t, "1234567890", phone)
}

func TestNormalizeEvent_Timestamp(t *testing.T) {
	// Create a timestamp with non-UTC timezone
	location, _ := time.LoadLocation("America/New_York")
	timestamp := time.Date(2024, 1, 15, 12, 0, 0, 0, location)
	
	event := &wguevents.TransformedEvent{
		BaseEvent: wguevents.BaseEvent{
			Timestamp: timestamp,
		},
	}
	
	normalizeEvent(event)
	
	assert.Equal(t, time.UTC, event.Timestamp.Location())
}

func TestEventValidator_Validate_EmptyEmail(t *testing.T) {
	validator := NewEventValidator()
	
	event := &wguevents.BaseEvent{
		EventID:      "test-event-123",
		EventType:    "user.created",
		SourceRegion: "us-west-2",
		Timestamp:    time.Now(),
		Metadata: wguevents.EventMetadata{
			SourceService: "user-service",
			TraceID:       "trace-123",
		},
		Payload: map[string]interface{}{
			"email": "",
		},
	}
	
	errors := validator.Validate(event)
	
	// Empty email should not trigger validation error (it's optional)
	for _, err := range errors {
		assert.NotEqual(t, "payload.email", err.Field)
	}
}

func TestEventValidator_Validate_NoEmail(t *testing.T) {
	validator := NewEventValidator()
	
	event := &wguevents.BaseEvent{
		EventID:      "test-event-123",
		EventType:    "user.created",
		SourceRegion: "us-west-2",
		Timestamp:    time.Now(),
		Metadata: wguevents.EventMetadata{
			SourceService: "user-service",
			TraceID:       "trace-123",
		},
		Payload: map[string]interface{}{},
	}
	
	errors := validator.Validate(event)
	
	// No email in payload should not trigger validation error
	for _, err := range errors {
		assert.NotEqual(t, "payload.email", err.Field)
	}
}

func TestEnrichEvent_DifferentRegions(t *testing.T) {
	ctx := context.Background()
	
	regions := []string{"us-west-2", "us-east-1", "eu-west-1", "ap-southeast-1", "unknown-region"}
	
	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			event := &wguevents.TransformedEvent{
				BaseEvent: wguevents.BaseEvent{
					SourceRegion: region,
				},
			}
			
			err := enrichEvent(ctx, event)
			
			assert.NoError(t, err)
			assert.NotNil(t, event.EnrichmentData)
			
			regionMetadata, ok := event.EnrichmentData["region_metadata"].(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, region, regionMetadata["region"])
		})
	}
}

func TestNormalizeEvent_NoEmailOrPhone(t *testing.T) {
	event := &wguevents.TransformedEvent{
		BaseEvent: wguevents.BaseEvent{
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"other_field": "value",
			},
		},
	}
	
	// Should not panic when email/phone not present
	assert.NotPanics(t, func() {
		normalizeEvent(event)
	})
}

func TestEventValidator_ValidationErrorCodes(t *testing.T) {
	validator := NewEventValidator()
	
	event := &wguevents.BaseEvent{
		// Empty event
	}
	
	errors := validator.Validate(event)
	
	assert.NotEmpty(t, errors)
	
	// Check that all required field errors have REQUIRED_FIELD code
	for _, err := range errors {
		if strings.Contains(err.Message, "required") {
			assert.Equal(t, "REQUIRED_FIELD", err.Code)
		}
	}
}
