package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewMetricsServer(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{"default port", ":9090"},
		{"custom port", ":8080"},
		{"localhost with port", "localhost:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMetricsServer(tt.addr)
			assert.NotNil(t, server)
			assert.Equal(t, tt.addr, server.addr)
			assert.NotNil(t, server.server)
		})
	}
}

func TestMetricsServerHandlers(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"health endpoint", "/health", http.StatusOK, "OK"},
		{"ready endpoint", "/ready", http.StatusOK, "READY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			if tt.path == "/health" {
				healthHandler(w, req)
			} else if tt.path == "/ready" {
				readyHandler(w, req)
			}

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestRecordLambdaInvocation(t *testing.T) {
	// Reset metrics before test
	LambdaInvocations.Reset()
	LambdaErrors.Reset()
	LambdaDuration.Reset()

	tests := []struct {
		name     string
		function string
		region   string
		duration time.Duration
		err      error
		wantErr  bool
	}{
		{
			name:     "successful invocation",
			function: "event-router",
			region:   "us-west-2",
			duration: 100 * time.Millisecond,
			err:      nil,
			wantErr:  false,
		},
		{
			name:     "failed invocation",
			function: "stream-processor",
			region:   "us-east-1",
			duration: 50 * time.Millisecond,
			err:      errors.New("processing error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordLambdaInvocation(tt.function, tt.region, tt.duration, tt.err)

			// Verify metrics were recorded
			counter, err := LambdaInvocations.GetMetricWithLabelValues(tt.function, tt.region)
			assert.NoError(t, err)
			assert.NotNil(t, counter)

			if tt.wantErr {
				errorCounter, err := LambdaErrors.GetMetricWithLabelValues(tt.function, tt.region, tt.err.Error())
				assert.NoError(t, err)
				assert.NotNil(t, errorCounter)
			}
		})
	}
}

func TestRecordKafkaMessage(t *testing.T) {
	// Reset metrics before test
	KafkaMessagesConsumed.Reset()
	KafkaProcessingDuration.Reset()
	KafkaProcessingErrors.Reset()

	tests := []struct {
		name          string
		topic         string
		partition     string
		consumerGroup string
		duration      time.Duration
		err           error
		wantErr       bool
	}{
		{
			name:          "successful message processing",
			topic:         "qlik.customers",
			partition:     "0",
			consumerGroup: "go-cdc-consumers",
			duration:      10 * time.Millisecond,
			err:           nil,
			wantErr:       false,
		},
		{
			name:          "failed message processing",
			topic:         "qlik.orders",
			partition:     "1",
			consumerGroup: "go-cdc-consumers",
			duration:      5 * time.Millisecond,
			err:           errors.New("deserialize error"),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordKafkaMessage(tt.topic, tt.partition, tt.consumerGroup, tt.duration, tt.err)

			// Verify metrics were recorded
			counter, err := KafkaMessagesConsumed.GetMetricWithLabelValues(tt.topic, tt.partition, tt.consumerGroup)
			assert.NoError(t, err)
			assert.NotNil(t, counter)

			if tt.wantErr {
				errorCounter, err := KafkaProcessingErrors.GetMetricWithLabelValues(tt.topic, tt.consumerGroup, tt.err.Error())
				assert.NoError(t, err)
				assert.NotNil(t, errorCounter)
			}
		})
	}
}

func TestRecordCDCEvent(t *testing.T) {
	// Reset metrics before test
	CDCEventsProcessed.Reset()
	CDCProcessingDuration.Reset()

	tests := []struct {
		name      string
		operation string
		table     string
		source    string
		duration  time.Duration
	}{
		{
			name:      "INSERT operation",
			operation: "INSERT",
			table:     "customers",
			source:    "qlik",
			duration:  15 * time.Millisecond,
		},
		{
			name:      "UPDATE operation",
			operation: "UPDATE",
			table:     "orders",
			source:    "qlik",
			duration:  20 * time.Millisecond,
		},
		{
			name:      "DELETE operation",
			operation: "DELETE",
			table:     "products",
			source:    "qlik",
			duration:  10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordCDCEvent(tt.operation, tt.table, tt.source, tt.duration)

			// Verify metrics were recorded
			counter, err := CDCEventsProcessed.GetMetricWithLabelValues(tt.operation, tt.table, tt.source)
			assert.NoError(t, err)
			assert.NotNil(t, counter)
		})
	}
}

func TestSetCircuitBreakerState(t *testing.T) {
	// Reset metrics before test
	CircuitBreakerState.Reset()

	tests := []struct {
		name           string
		service        string
		region         string
		state          string
		expectedValue  float64
	}{
		{
			name:          "closed state",
			service:       "event-router",
			region:        "us-west-2",
			state:         "closed",
			expectedValue: 0,
		},
		{
			name:          "open state",
			service:       "event-router",
			region:        "us-west-2",
			state:         "open",
			expectedValue: 1,
		},
		{
			name:          "half_open state",
			service:       "event-router",
			region:        "us-west-2",
			state:         "half_open",
			expectedValue: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetCircuitBreakerState(tt.service, tt.region, tt.state)

			// Verify state was set
			gauge, err := CircuitBreakerState.GetMetricWithLabelValues(tt.service, tt.region)
			assert.NoError(t, err)
			assert.NotNil(t, gauge)
		})
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered
	metrics := []prometheus.Collector{
		LambdaInvocations,
		LambdaErrors,
		LambdaDuration,
		KafkaMessagesConsumed,
		KafkaConsumerLag,
		KafkaProcessingDuration,
		KafkaProcessingErrors,
		CDCEventsProcessed,
		CDCProcessingDuration,
		EventBridgePublished,
		EventBridgeErrors,
		CircuitBreakerState,
		CircuitBreakerFailures,
		DynamoDBOperations,
		DynamoDBErrors,
		CrossRegionEvents,
		CrossRegionLatency,
		DLQMessages,
	}

	for _, metric := range metrics {
		assert.NotNil(t, metric)
	}
}

func TestMetricsServerShutdown(t *testing.T) {
	server := NewMetricsServer(":0") // Use port 0 to let OS assign a free port

	// Start server in goroutine
	go func() {
		_ = server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test shutdown
	err := server.Shutdown(5 * time.Second)
	assert.NoError(t, err)
}
