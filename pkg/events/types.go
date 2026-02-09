package events

import (
	"encoding/json"
	"time"
)

// BaseEvent represents the core event structure for all events in the system
type BaseEvent struct {
	EventID       string                 `json:"event_id"`
	EventType     string                 `json:"event_type"`
	SourceRegion  string                 `json:"source_region"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Metadata      EventMetadata          `json:"metadata"`
	Payload       map[string]interface{} `json:"payload"`
}

// EventMetadata contains contextual information about the event
type EventMetadata struct {
	SourceService string `json:"source_service"`
	UserID        string `json:"user_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	TraceID       string `json:"trace_id"`
	Version       string `json:"version"`
	Priority      int    `json:"priority,omitempty"`
}

// CrossRegionEvent wraps a BaseEvent for cross-region transmission
type CrossRegionEvent struct {
	BaseEvent
	TargetRegion      string    `json:"target_region"`
	OriginalTimestamp time.Time `json:"original_timestamp"`
	CompressionType   string    `json:"compression_type,omitempty"`
	Checksum          string    `json:"checksum,omitempty"`
}

// CDCEvent represents a Change Data Capture event from Qlik
type CDCEvent struct {
	Operation     string                 `json:"operation"` // INSERT, UPDATE, DELETE, REFRESH
	TableName     string                 `json:"table_name"`
	Schema        string                 `json:"schema"`
	Timestamp     time.Time              `json:"timestamp"`
	TransactionID string                 `json:"transaction_id,omitempty"`
	Before        map[string]interface{} `json:"before,omitempty"`
	After         map[string]interface{} `json:"after,omitempty"`
	PrimaryKeys   map[string]interface{} `json:"primary_keys"`
	Metadata      CDCMetadata            `json:"metadata"`
}

// CDCMetadata contains metadata specific to CDC events
type CDCMetadata struct {
	SourceDatabase string    `json:"source_database"`
	SourceTable    string    `json:"source_table"`
	LSN            string    `json:"lsn,omitempty"`           // Log Sequence Number
	SCN            string    `json:"scn,omitempty"`           // System Change Number
	Offset         int64     `json:"offset"`
	Partition      int32     `json:"partition"`
	CaptureTime    time.Time `json:"capture_time"`
	ApplyTime      time.Time `json:"apply_time,omitempty"`
}

// EventRecord represents a DynamoDB Stream record
type EventRecord struct {
	EventID      string                 `json:"event_id"`
	EventName    string                 `json:"event_name"` // INSERT, MODIFY, REMOVE
	EventVersion string                 `json:"event_version"`
	EventSource  string                 `json:"event_source"`
	AWSRegion    string                 `json:"aws_region"`
	DynamoDB     DynamoDBStreamRecord   `json:"dynamodb"`
	UserIdentity map[string]interface{} `json:"user_identity,omitempty"`
}

// DynamoDBStreamRecord contains DynamoDB-specific stream data
type DynamoDBStreamRecord struct {
	ApproximateCreationDateTime time.Time              `json:"approximate_creation_date_time"`
	Keys                        map[string]interface{} `json:"keys"`
	NewImage                    map[string]interface{} `json:"new_image,omitempty"`
	OldImage                    map[string]interface{} `json:"old_image,omitempty"`
	SequenceNumber              string                 `json:"sequence_number"`
	SizeBytes                   int64                  `json:"size_bytes"`
	StreamViewType              string                 `json:"stream_view_type"`
}

// HealthCheckEvent represents a health check across regions
type HealthCheckEvent struct {
	Region        string            `json:"region"`
	Service       string            `json:"service"`
	Status        string            `json:"status"` // healthy, degraded, unhealthy
	Timestamp     time.Time         `json:"timestamp"`
	Dependencies  []DependencyCheck `json:"dependencies"`
	Metrics       HealthMetrics     `json:"metrics"`
	ErrorMessages []string          `json:"error_messages,omitempty"`
}

// DependencyCheck represents the status of a dependency
type DependencyCheck struct {
	Name      string        `json:"name"`
	Type      string        `json:"type"` // database, kafka, api, cache
	Status    string        `json:"status"`
	Latency   time.Duration `json:"latency"`
	ErrorRate float64       `json:"error_rate"`
}

// HealthMetrics contains performance metrics for health checks
type HealthMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	Latency     int64   `json:"latency_ms"`
	Throughput  int64   `json:"throughput_rps"`
	ErrorRate   float64 `json:"error_rate"`
}

// DeadLetterEvent wraps events that failed processing
type DeadLetterEvent struct {
	OriginalEvent json.RawMessage `json:"original_event"`
	ErrorMessage  string          `json:"error_message"`
	ErrorType     string          `json:"error_type"`
	FailureCount  int             `json:"failure_count"`
	FirstFailure  time.Time       `json:"first_failure"`
	LastFailure   time.Time       `json:"last_failure"`
	SourceHandler string          `json:"source_handler"`
	StackTrace    string          `json:"stack_trace,omitempty"`
}

// TransformedEvent represents an event after transformation/enrichment
type TransformedEvent struct {
	BaseEvent
	TransformationRules []string               `json:"transformation_rules"`
	EnrichmentData      map[string]interface{} `json:"enrichment_data,omitempty"`
	ValidationErrors    []ValidationError      `json:"validation_errors,omitempty"`
	TransformedAt       time.Time              `json:"transformed_at"`
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// EventBatch represents a batch of events for bulk processing
type EventBatch struct {
	BatchID   string      `json:"batch_id"`
	Events    []BaseEvent `json:"events"`
	Timestamp time.Time   `json:"timestamp"`
	Size      int         `json:"size"`
	Region    string      `json:"region"`
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState struct {
	State             string    `json:"state"` // closed, open, half_open
	FailureCount      int       `json:"failure_count"`
	SuccessCount      int       `json:"success_count"`
	LastFailureTime   time.Time `json:"last_failure_time,omitempty"`
	LastStateChange   time.Time `json:"last_state_change"`
	ConsecutiveErrors int       `json:"consecutive_errors"`
	NextRetryTime     time.Time `json:"next_retry_time,omitempty"`
}

// EventType constants
const (
	EventTypeCustomerCreated    = "customer.created"
	EventTypeCustomerUpdated    = "customer.updated"
	EventTypeCustomerDeleted    = "customer.deleted"
	EventTypeOrderPlaced        = "order.placed"
	EventTypeOrderFulfilled     = "order.fulfilled"
	EventTypePaymentProcessed   = "payment.processed"
	EventTypeInventoryUpdated   = "inventory.updated"
	EventTypeCrossRegion        = "cross_region.event"
	EventTypeHealthCheck        = "health.check"
	EventTypeCircuitBreakerOpen = "circuit_breaker.open"
)

// Operation types for CDC
const (
	OperationInsert  = "INSERT"
	OperationUpdate  = "UPDATE"
	OperationDelete  = "DELETE"
	OperationRefresh = "REFRESH"
)

// Health status constants
const (
	StatusHealthy   = "healthy"
	StatusDegraded  = "degraded"
	StatusUnhealthy = "unhealthy"
)

// Circuit breaker states
const (
	CircuitBreakerClosed   = "closed"
	CircuitBreakerOpen     = "open"
	CircuitBreakerHalfOpen = "half_open"
)

// NewBaseEvent creates a new BaseEvent with generated ID and timestamp
func NewBaseEvent(eventType, sourceRegion string, payload map[string]interface{}) *BaseEvent {
	return &BaseEvent{
		EventID:      generateEventID(),
		EventType:    eventType,
		SourceRegion: sourceRegion,
		Timestamp:    time.Now(),
		Payload:      payload,
		Metadata: EventMetadata{
			Version: "1.0",
		},
	}
}

// NewCDCEvent creates a new CDC event
func NewCDCEvent(operation, tableName string, after, before map[string]interface{}) *CDCEvent {
	return &CDCEvent{
		Operation: operation,
		TableName: tableName,
		Timestamp: time.Now(),
		After:     after,
		Before:    before,
		Metadata: CDCMetadata{
			CaptureTime: time.Now(),
		},
	}
}

// ToJSON serializes an event to JSON
func (e *BaseEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes a BaseEvent from JSON
func FromJSON(data []byte) (*BaseEvent, error) {
	var event BaseEvent
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// generateEventID generates a unique event ID
func generateEventID() string {
	// Simple implementation - in production use ULID or similar
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}
