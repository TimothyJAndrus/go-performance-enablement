package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/klauspost/compress/zstd"
	"github.com/wgu/go-performance-enablement/pkg/awsutils"
	"github.com/wgu/go-performance-enablement/pkg/events" as wguevents
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

var (
	logger           *zap.Logger
	awsClients       *awsutils.AWSClients
	partnerClients   *awsutils.AWSClients
	publisher        *awsutils.EventBridgePublisher
	circuitBreaker   *CircuitBreaker
	currentRegion    string
	partnerRegion    string
	eventBusName     string
	dlqURL           string
)

func init() {
	var err error
	
	// Initialize logger
	logger, _ = zap.NewProduction()
	
	// Get environment variables
	currentRegion = os.Getenv("AWS_REGION")
	partnerRegion = os.Getenv("PARTNER_REGION")
	eventBusName = os.Getenv("EVENT_BUS_NAME")
	dlqURL = os.Getenv("DLQ_URL")
	
	// Initialize AWS clients for current region
	ctx := context.Background()
	awsClients, err = awsutils.NewAWSClients(ctx)
	if err != nil {
		logger.Fatal("failed to create AWS clients", zap.Error(err))
	}
	
	// Initialize AWS clients for partner region
	partnerClients, err = awsutils.NewAWSClientsWithRegion(ctx, partnerRegion)
	if err != nil {
		logger.Fatal("failed to create partner AWS clients", zap.Error(err))
	}
	
	// Initialize EventBridge publisher
	publisher = awsutils.NewEventBridgePublisher(
		partnerClients.EventBridge,
		eventBusName,
		"event-router",
	)
	
	// Initialize circuit breaker
	circuitBreaker = NewCircuitBreaker(5, 30*time.Second)
}

// Handler processes events and routes them to the partner region
func Handler(ctx context.Context, event events.DynamoDBEvent) error {
	start := time.Now()
	functionName := "event-router"
	
	logger.Info("processing event batch",
		zap.Int("record_count", len(event.Records)),
		zap.String("source_region", currentRegion),
		zap.String("target_region", partnerRegion),
	)
	
	var errors []error
	
	for _, record := range event.Records {
		if err := processRecord(ctx, record); err != nil {
			errors = append(errors, err)
			logger.Error("failed to process record",
				zap.Error(err),
				zap.String("event_id", record.EventID),
			)
		}
	}
	
	duration := time.Since(start)
	
	var finalErr error
	if len(errors) > 0 {
		finalErr = fmt.Errorf("failed to process %d/%d records", len(errors), len(event.Records))
	}
	
	metrics.RecordLambdaInvocation(functionName, currentRegion, duration, finalErr)
	
	if finalErr != nil {
		return finalErr
	}
	
	logger.Info("successfully processed event batch",
		zap.Duration("duration", duration),
		zap.Int("record_count", len(event.Records)),
	)
	
	return nil
}

func processRecord(ctx context.Context, record events.DynamoDBEventRecord) error {
	// Parse the DynamoDB record into our event structure
	baseEvent, err := parseRecord(record)
	if err != nil {
		return fmt.Errorf("failed to parse record: %w", err)
	}
	
	// Create cross-region event
	crossRegionEvent := &wguevents.CrossRegionEvent{
		BaseEvent:         *baseEvent,
		TargetRegion:      partnerRegion,
		OriginalTimestamp: baseEvent.Timestamp,
		CompressionType:   "zstd",
	}
	
	// Compress event payload
	compressedPayload, err := compressEvent(crossRegionEvent)
	if err != nil {
		logger.Warn("failed to compress event, sending uncompressed",
			zap.Error(err),
			zap.String("event_id", baseEvent.EventID),
		)
		crossRegionEvent.CompressionType = "none"
	} else {
		crossRegionEvent.Payload = map[string]interface{}{
			"compressed_data": compressedPayload,
		}
	}
	
	// Route through circuit breaker
	err = circuitBreaker.Execute(func() error {
		return publisher.PublishCrossRegionEvent(ctx, partnerRegion, crossRegionEvent)
	})
	
	if err != nil {
		// Send to DLQ
		if dlqErr := sendToDLQ(ctx, baseEvent, err); dlqErr != nil {
			logger.Error("failed to send to DLQ",
				zap.Error(dlqErr),
				zap.String("event_id", baseEvent.EventID),
			)
		}
		
		metrics.CrossRegionEvents.WithLabelValues(currentRegion, partnerRegion).Inc()
		return fmt.Errorf("failed to route event: %w", err)
	}
	
	// Record successful routing
	latency := time.Since(crossRegionEvent.OriginalTimestamp)
	metrics.CrossRegionLatency.WithLabelValues(currentRegion, partnerRegion).Observe(latency.Seconds())
	metrics.CrossRegionEvents.WithLabelValues(currentRegion, partnerRegion).Inc()
	
	logger.Debug("successfully routed event",
		zap.String("event_id", baseEvent.EventID),
		zap.String("event_type", baseEvent.EventType),
		zap.Duration("latency", latency),
	)
	
	return nil
}

func parseRecord(record events.DynamoDBEventRecord) (*wguevents.BaseEvent, error) {
	// Convert DynamoDB attribute values to BaseEvent
	payload := make(map[string]interface{})
	
	for key, value := range record.Change.NewImage {
		payload[key] = value
	}
	
	event := wguevents.NewBaseEvent(
		record.EventName,
		currentRegion,
		payload,
	)
	
	event.EventID = record.EventID
	event.Metadata.SourceService = "dynamodb-streams"
	
	return event, nil
}

func compressEvent(event *wguevents.CrossRegionEvent) ([]byte, error) {
	// Serialize event to JSON
	jsonData, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Compress with zstd
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor: %w", err)
	}
	
	compressed := encoder.EncodeAll(jsonData, make([]byte, 0, len(jsonData)))
	
	compressionRatio := float64(len(jsonData)) / float64(len(compressed))
	logger.Debug("compressed event",
		zap.Int("original_size", len(jsonData)),
		zap.Int("compressed_size", len(compressed)),
		zap.Float64("compression_ratio", compressionRatio),
	)
	
	return compressed, nil
}

func sendToDLQ(ctx context.Context, event *wguevents.BaseEvent, processingError error) error {
	dlqEvent := &wguevents.DeadLetterEvent{
		ErrorMessage:  processingError.Error(),
		ErrorType:     "routing_failure",
		FailureCount:  1,
		FirstFailure:  time.Now(),
		LastFailure:   time.Now(),
		SourceHandler: "event-router",
	}
	
	originalJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal original event: %w", err)
	}
	dlqEvent.OriginalEvent = originalJSON
	
	messageBody, err := json.Marshal(dlqEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ event: %w", err)
	}
	
	err = awsClients.SendToDeadLetterQueue(ctx, dlqURL, string(messageBody), processingError.Error())
	if err != nil {
		return fmt.Errorf("failed to send to DLQ: %w", err)
	}
	
	metrics.DLQMessages.WithLabelValues("event-router", "routing_failure").Inc()
	
	return nil
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures    int
	timeout        time.Duration
	state          string
	failureCount   int
	successCount   int
	lastFailure    time.Time
	lastStateChange time.Time
	mu             sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:     maxFailures,
		timeout:         timeout,
		state:           wguevents.CircuitBreakerClosed,
		lastStateChange: time.Now(),
	}
}

// Execute runs the function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// Check if circuit is open
	if cb.state == wguevents.CircuitBreakerOpen {
		if time.Since(cb.lastStateChange) > cb.timeout {
			// Transition to half-open
			cb.state = wguevents.CircuitBreakerHalfOpen
			cb.successCount = 0
			cb.lastStateChange = time.Now()
			metrics.SetCircuitBreakerState("cross-region", currentRegion, cb.state)
			logger.Info("circuit breaker transitioning to half-open")
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}
	
	// Execute function
	err := fn()
	
	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		metrics.CircuitBreakerFailures.WithLabelValues("cross-region", currentRegion).Inc()
		
		if cb.state == wguevents.CircuitBreakerHalfOpen {
			// Go back to open on any failure in half-open
			cb.state = wguevents.CircuitBreakerOpen
			cb.lastStateChange = time.Now()
			metrics.SetCircuitBreakerState("cross-region", currentRegion, cb.state)
			logger.Warn("circuit breaker opened",
				zap.Int("failure_count", cb.failureCount),
			)
		} else if cb.failureCount >= cb.maxFailures {
			// Open circuit
			cb.state = wguevents.CircuitBreakerOpen
			cb.lastStateChange = time.Now()
			metrics.SetCircuitBreakerState("cross-region", currentRegion, cb.state)
			logger.Warn("circuit breaker opened",
				zap.Int("failure_count", cb.failureCount),
			)
		}
		
		return err
	}
	
	// Success
	cb.successCount++
	
	if cb.state == wguevents.CircuitBreakerHalfOpen {
		// After successful attempt in half-open, close circuit
		if cb.successCount >= 2 {
			cb.state = wguevents.CircuitBreakerClosed
			cb.failureCount = 0
			cb.lastStateChange = time.Now()
			metrics.SetCircuitBreakerState("cross-region", currentRegion, cb.state)
			logger.Info("circuit breaker closed")
		}
	}
	
	return nil
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func main() {
	lambda.Start(Handler)
}
