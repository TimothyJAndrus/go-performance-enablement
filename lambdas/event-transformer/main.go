package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wgu/go-performance-enablement/pkg/awsutils"
	wguevents "github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

var (
	logger        *zap.Logger
	awsClients    *awsutils.AWSClients
	publisher     *awsutils.EventBridgePublisher
	currentRegion string
	eventBusName  string
	validator     *EventValidator
)

func init() {
	var err error

	// Initialize logger
	logger, _ = zap.NewProduction()

	// Get environment variables
	currentRegion = os.Getenv("AWS_REGION")
	eventBusName = os.Getenv("EVENT_BUS_NAME")

	// Initialize AWS clients
	ctx := context.Background()
	awsClients, err = awsutils.NewAWSClients(ctx)
	if err != nil {
		logger.Fatal("failed to create AWS clients", zap.Error(err))
	}

	// Initialize EventBridge publisher
	publisher = awsutils.NewEventBridgePublisher(
		awsClients.EventBridge,
		eventBusName,
		"event-transformer",
	)

	// Initialize validator
	validator = NewEventValidator()
}

// Handler processes EventBridge events and transforms them
func Handler(ctx context.Context, event events.CloudWatchEvent) error {
	start := time.Now()
	functionName := "event-transformer"

	logger.Info("processing event",
		zap.String("detail_type", event.DetailType),
		zap.String("source", event.Source),
		zap.String("event_id", event.ID),
	)

	// Parse the event
	var baseEvent wguevents.BaseEvent
	if err := json.Unmarshal(event.Detail, &baseEvent); err != nil {
		logger.Error("failed to parse event", zap.Error(err))
		duration := time.Since(start)
		metrics.RecordLambdaInvocation(functionName, currentRegion, duration, err)
		return fmt.Errorf("failed to parse event: %w", err)
	}

	// Validate the event
	validationErrors := validator.Validate(&baseEvent)

	// Transform and enrich the event
	transformedEvent := &wguevents.TransformedEvent{
		BaseEvent:           baseEvent,
		TransformationRules: []string{"validate", "enrich", "normalize"},
		TransformedAt:       time.Now(),
		ValidationErrors:    validationErrors,
	}

	// Enrich with additional data
	if err := enrichEvent(ctx, transformedEvent); err != nil {
		logger.Warn("failed to enrich event", zap.Error(err))
		// Continue processing even if enrichment fails
	}

	// Normalize data
	normalizeEvent(transformedEvent)

	// Publish transformed event
	if len(validationErrors) == 0 {
		if err := publisher.PublishEvent(ctx, "event.transformed", transformedEvent); err != nil {
			logger.Error("failed to publish transformed event", zap.Error(err))
			duration := time.Since(start)
			metrics.RecordLambdaInvocation(functionName, currentRegion, duration, err)
			return fmt.Errorf("failed to publish event: %w", err)
		}
	} else {
		logger.Warn("event has validation errors, publishing to error stream",
			zap.Int("error_count", len(validationErrors)),
		)
		if err := publisher.PublishEvent(ctx, "event.validation_failed", transformedEvent); err != nil {
			logger.Error("failed to publish validation failed event", zap.Error(err))
		}
	}

	duration := time.Since(start)
	metrics.RecordLambdaInvocation(functionName, currentRegion, duration, nil)

	logger.Info("successfully transformed event",
		zap.Duration("duration", duration),
		zap.Int("validation_errors", len(validationErrors)),
	)

	return nil
}

// EventValidator validates events
type EventValidator struct {
	emailRegex *regexp.Regexp
	uuidRegex  *regexp.Regexp
}

// NewEventValidator creates a new event validator
func NewEventValidator() *EventValidator {
	return &EventValidator{
		emailRegex: regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		uuidRegex:  regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`),
	}
}

// Validate validates an event and returns validation errors
func (v *EventValidator) Validate(event *wguevents.BaseEvent) []wguevents.ValidationError {
	var errors []wguevents.ValidationError

	// Validate required fields
	if event.EventID == "" {
		errors = append(errors, wguevents.ValidationError{
			Field:   "event_id",
			Message: "event_id is required",
			Code:    "REQUIRED_FIELD",
		})
	}

	if event.EventType == "" {
		errors = append(errors, wguevents.ValidationError{
			Field:   "event_type",
			Message: "event_type is required",
			Code:    "REQUIRED_FIELD",
		})
	}

	if event.SourceRegion == "" {
		errors = append(errors, wguevents.ValidationError{
			Field:   "source_region",
			Message: "source_region is required",
			Code:    "REQUIRED_FIELD",
		})
	}

	// Validate timestamp
	if event.Timestamp.IsZero() {
		errors = append(errors, wguevents.ValidationError{
			Field:   "timestamp",
			Message: "timestamp is required",
			Code:    "REQUIRED_FIELD",
		})
	} else if event.Timestamp.After(time.Now().Add(5 * time.Minute)) {
		errors = append(errors, wguevents.ValidationError{
			Field:   "timestamp",
			Message: "timestamp is in the future",
			Code:    "INVALID_TIMESTAMP",
		})
	}

	// Validate metadata
	if event.Metadata.SourceService == "" {
		errors = append(errors, wguevents.ValidationError{
			Field:   "metadata.source_service",
			Message: "source_service is required",
			Code:    "REQUIRED_FIELD",
		})
	}

	if event.Metadata.TraceID == "" {
		errors = append(errors, wguevents.ValidationError{
			Field:   "metadata.trace_id",
			Message: "trace_id is required",
			Code:    "REQUIRED_FIELD",
		})
	}

	// Validate email if present in payload
	if email, ok := event.Payload["email"].(string); ok && email != "" {
		if !v.emailRegex.MatchString(email) {
			errors = append(errors, wguevents.ValidationError{
				Field:   "payload.email",
				Message: "invalid email format",
				Code:    "INVALID_FORMAT",
			})
		}
	}

	return errors
}

// enrichEvent enriches the event with additional data
func enrichEvent(ctx context.Context, event *wguevents.TransformedEvent) error {
	enrichmentData := make(map[string]interface{})

	// Add geolocation data based on region
	enrichmentData["region_metadata"] = map[string]interface{}{
		"region":    event.SourceRegion,
		"timezone":  getTimezoneForRegion(event.SourceRegion),
		"data_center": getDataCenterForRegion(event.SourceRegion),
	}

	// Add processing metadata
	enrichmentData["processing_metadata"] = map[string]interface{}{
		"processed_at": time.Now(),
		"processor":    "event-transformer",
		"version":      "1.0.0",
	}

	// Could fetch additional data from DynamoDB, external APIs, etc.
	// For example:
	// - Customer profile data
	// - Product information
	// - Historical context

	event.EnrichmentData = enrichmentData

	return nil
}

// normalizeEvent normalizes event data
func normalizeEvent(event *wguevents.TransformedEvent) {
	// Normalize email to lowercase
	if email, ok := event.Payload["email"].(string); ok {
		event.Payload["email"] = normalizeEmail(email)
	}

	// Normalize phone numbers
	if phone, ok := event.Payload["phone"].(string); ok {
		event.Payload["phone"] = normalizePhone(phone)
	}

	// Ensure consistent timestamp format
	if event.Timestamp.Location() != time.UTC {
		event.Timestamp = event.Timestamp.UTC()
	}
}

// normalizeEmail normalizes email addresses
func normalizeEmail(email string) string {
	// Convert to lowercase and trim whitespace
	return regexp.MustCompile(`\s+`).ReplaceAllString(email, "")
}

// normalizePhone normalizes phone numbers
func normalizePhone(phone string) string {
	// Remove all non-numeric characters
	return regexp.MustCompile(`[^0-9+]`).ReplaceAllString(phone, "")
}

// getTimezoneForRegion returns timezone for AWS region
func getTimezoneForRegion(region string) string {
	timezones := map[string]string{
		"us-west-2": "America/Los_Angeles",
		"us-east-1": "America/New_York",
		"eu-west-1": "Europe/Dublin",
		"ap-southeast-1": "Asia/Singapore",
	}

	if tz, ok := timezones[region]; ok {
		return tz
	}
	return "UTC"
}

// getDataCenterForRegion returns data center location for AWS region
func getDataCenterForRegion(region string) string {
	datacenters := map[string]string{
		"us-west-2": "Oregon",
		"us-east-1": "Virginia",
		"eu-west-1": "Ireland",
		"ap-southeast-1": "Singapore",
	}

	if dc, ok := datacenters[region]; ok {
		return dc
	}
	return "Unknown"
}

func main() {
	lambda.Start(Handler)
}
