package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Lambda metrics
	LambdaInvocations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lambda_invocations_total",
			Help: "Total number of Lambda invocations",
		},
		[]string{"function", "region"},
	)

	LambdaErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lambda_errors_total",
			Help: "Total number of Lambda errors",
		},
		[]string{"function", "region", "error_type"},
	)

	LambdaDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lambda_duration_seconds",
			Help:    "Lambda execution duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"function", "region"},
	)

	// Kafka consumer metrics
	KafkaMessagesConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_consumed_total",
			Help: "Total number of Kafka messages consumed",
		},
		[]string{"topic", "partition", "consumer_group"},
	)

	KafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_consumer_lag_seconds",
			Help: "Kafka consumer lag in seconds",
		},
		[]string{"topic", "partition", "consumer_group"},
	)

	KafkaProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_processing_duration_seconds",
			Help:    "Kafka message processing duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"topic", "consumer_group"},
	)

	KafkaProcessingErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_processing_errors_total",
			Help: "Total number of Kafka processing errors",
		},
		[]string{"topic", "consumer_group", "error_type"},
	)

	// CDC metrics
	CDCEventsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdc_events_processed_total",
			Help: "Total number of CDC events processed",
		},
		[]string{"operation", "table", "source"},
	)

	CDCProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cdc_processing_duration_seconds",
			Help:    "CDC event processing duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5},
		},
		[]string{"operation", "table"},
	)

	// EventBridge metrics
	EventBridgePublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventbridge_events_published_total",
			Help: "Total number of EventBridge events published",
		},
		[]string{"event_type", "region"},
	)

	EventBridgeErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventbridge_errors_total",
			Help: "Total number of EventBridge publishing errors",
		},
		[]string{"event_type", "region", "error_type"},
	)

	// Circuit breaker metrics
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half_open)",
		},
		[]string{"service", "region"},
	)

	CircuitBreakerFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "circuit_breaker_failures_total",
			Help: "Total number of circuit breaker failures",
		},
		[]string{"service", "region"},
	)

	// DynamoDB metrics
	DynamoDBOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dynamodb_operations_total",
			Help: "Total number of DynamoDB operations",
		},
		[]string{"table", "operation", "region"},
	)

	DynamoDBErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dynamodb_errors_total",
			Help: "Total number of DynamoDB errors",
		},
		[]string{"table", "operation", "region", "error_type"},
	)

	// Cross-region replication metrics
	CrossRegionEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cross_region_events_total",
			Help: "Total number of cross-region events",
		},
		[]string{"source_region", "target_region"},
	)

	CrossRegionLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cross_region_latency_seconds",
			Help:    "Cross-region replication latency in seconds",
			Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"source_region", "target_region"},
	)

	// Dead letter queue metrics
	DLQMessages = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlq_messages_total",
			Help: "Total number of messages sent to DLQ",
		},
		[]string{"source", "error_type"},
	)
)

// MetricsServer provides HTTP endpoint for Prometheus metrics
type MetricsServer struct {
	addr   string
	server *http.Server
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(addr string) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)

	return &MetricsServer{
		addr: addr,
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Start starts the metrics server
func (s *MetricsServer) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the metrics server
func (s *MetricsServer) Shutdown(timeout time.Duration) error {
	ctx, cancel := WithTimeout(nil, timeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler handles readiness check requests
func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("READY"))
}

// RecordLambdaInvocation records a Lambda invocation
func RecordLambdaInvocation(function, region string, duration time.Duration, err error) {
	LambdaInvocations.WithLabelValues(function, region).Inc()
	LambdaDuration.WithLabelValues(function, region).Observe(duration.Seconds())

	if err != nil {
		errorType := "unknown"
		if err != nil {
			errorType = err.Error()
		}
		LambdaErrors.WithLabelValues(function, region, errorType).Inc()
	}
}

// RecordKafkaMessage records Kafka message processing
func RecordKafkaMessage(topic, partition, consumerGroup string, duration time.Duration, err error) {
	KafkaMessagesConsumed.WithLabelValues(topic, partition, consumerGroup).Inc()
	KafkaProcessingDuration.WithLabelValues(topic, consumerGroup).Observe(duration.Seconds())

	if err != nil {
		errorType := "unknown"
		if err != nil {
			errorType = err.Error()
		}
		KafkaProcessingErrors.WithLabelValues(topic, consumerGroup, errorType).Inc()
	}
}

// RecordCDCEvent records CDC event processing
func RecordCDCEvent(operation, table, source string, duration time.Duration) {
	CDCEventsProcessed.WithLabelValues(operation, table, source).Inc()
	CDCProcessingDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// SetCircuitBreakerState sets the circuit breaker state metric
func SetCircuitBreakerState(service, region, state string) {
	var stateValue float64
	switch state {
	case "closed":
		stateValue = 0
	case "open":
		stateValue = 1
	case "half_open":
		stateValue = 2
	}
	CircuitBreakerState.WithLabelValues(service, region).Set(stateValue)
}
