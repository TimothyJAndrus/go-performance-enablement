package main

import (
	"testing"
	"time"

	wguevents "github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestDetermineHealthStatus_AllHealthy(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Name: "dynamodb", Status: wguevents.StatusHealthy},
		{Name: "eventbridge", Status: wguevents.StatusHealthy},
		{Name: "sqs", Status: wguevents.StatusHealthy},
	}

	status := determineHealthStatus(dependencies)
	assert.Equal(t, wguevents.StatusHealthy, status)
}

func TestDetermineHealthStatus_SomeDegraded(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Name: "dynamodb", Status: wguevents.StatusHealthy},
		{Name: "eventbridge", Status: wguevents.StatusDegraded},
		{Name: "sqs", Status: wguevents.StatusHealthy},
	}

	status := determineHealthStatus(dependencies)
	assert.Equal(t, wguevents.StatusDegraded, status)
}

func TestDetermineHealthStatus_SomeUnhealthy(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Name: "dynamodb", Status: wguevents.StatusUnhealthy},
		{Name: "eventbridge", Status: wguevents.StatusHealthy},
		{Name: "sqs", Status: wguevents.StatusDegraded},
	}

	status := determineHealthStatus(dependencies)
	assert.Equal(t, wguevents.StatusUnhealthy, status, "Unhealthy should take precedence")
}

func TestDetermineHealthStatus_Empty(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{}

	status := determineHealthStatus(dependencies)
	assert.Equal(t, wguevents.StatusHealthy, status, "Empty dependencies should default to healthy")
}

func TestCalculateMetrics_BasicAverages(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Name: "service1", Latency: 100 * time.Millisecond, ErrorRate: 0.01},
		{Name: "service2", Latency: 200 * time.Millisecond, ErrorRate: 0.02},
		{Name: "service3", Latency: 300 * time.Millisecond, ErrorRate: 0.03},
	}

	metrics := calculateMetrics(dependencies)

	// Average latency: (100 + 200 + 300) / 3 = 200ms
	assert.Equal(t, int64(200), metrics.Latency)
	// Average error rate: (0.01 + 0.02 + 0.03) / 3 = 0.02
	assert.InDelta(t, 0.02, metrics.ErrorRate, 0.001)
}

func TestCalculateMetrics_Empty(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{}

	metrics := calculateMetrics(dependencies)

	assert.Equal(t, int64(0), metrics.Latency)
	assert.Equal(t, 0.0, metrics.ErrorRate)
}

func TestCalculateMetrics_ZeroValues(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Name: "service1", Latency: 0, ErrorRate: 0.0},
		{Name: "service2", Latency: 0, ErrorRate: 0.0},
	}

	metrics := calculateMetrics(dependencies)

	assert.Equal(t, int64(0), metrics.Latency)
	assert.Equal(t, 0.0, metrics.ErrorRate)
}

func TestAggregateHealth_EmptyChecks(t *testing.T) {
	checks := []*wguevents.HealthCheckEvent{}

	aggregated := aggregateHealth(checks)

	assert.NotNil(t, aggregated)
	assert.Equal(t, wguevents.StatusUnhealthy, aggregated.Status)
	assert.Equal(t, "multi-region-eda", aggregated.Service)
}

func TestAggregateHealth_SingleRegion(t *testing.T) {
	checks := []*wguevents.HealthCheckEvent{
		{
			Region:  "us-west-2",
			Service: "multi-region-eda",
			Status:  wguevents.StatusHealthy,
			Dependencies: []wguevents.DependencyCheck{
				{Name: "dynamodb", Status: wguevents.StatusHealthy, Latency: 100 * time.Millisecond},
			},
			ErrorMessages: []string{},
			Timestamp:     time.Now(),
		},
	}

	aggregated := aggregateHealth(checks)

	assert.NotNil(t, aggregated)
	assert.Equal(t, "multi-region", aggregated.Region)
	assert.Equal(t, wguevents.StatusHealthy, aggregated.Status)
	assert.Len(t, aggregated.Dependencies, 1)
}

func TestAggregateHealth_MultipleRegions(t *testing.T) {
	checks := []*wguevents.HealthCheckEvent{
		{
			Region:  "us-west-2",
			Service: "multi-region-eda",
			Status:  wguevents.StatusHealthy,
			Dependencies: []wguevents.DependencyCheck{
				{Name: "dynamodb", Status: wguevents.StatusHealthy, Latency: 100 * time.Millisecond},
				{Name: "eventbridge", Status: wguevents.StatusHealthy, Latency: 150 * time.Millisecond},
			},
			ErrorMessages: []string{},
			Timestamp:     time.Now(),
		},
		{
			Region:  "us-east-1",
			Service: "multi-region-eda",
			Status:  wguevents.StatusDegraded,
			Dependencies: []wguevents.DependencyCheck{
				{Name: "dynamodb", Status: wguevents.StatusDegraded, Latency: 600 * time.Millisecond},
				{Name: "sqs", Status: wguevents.StatusHealthy, Latency: 200 * time.Millisecond},
			},
			ErrorMessages: []string{"DynamoDB: degraded"},
			Timestamp:     time.Now(),
		},
	}

	aggregated := aggregateHealth(checks)

	assert.NotNil(t, aggregated)
	assert.Equal(t, "multi-region", aggregated.Region)
	// Should be degraded because one region has degraded status
	assert.Equal(t, wguevents.StatusDegraded, aggregated.Status)
	// Should have all dependencies from both regions
	assert.Len(t, aggregated.Dependencies, 4)
	// Should collect all error messages
	assert.Len(t, aggregated.ErrorMessages, 1)
	assert.Contains(t, aggregated.ErrorMessages, "DynamoDB: degraded")
}

func TestAggregateHealth_WorstStatusWins(t *testing.T) {
	checks := []*wguevents.HealthCheckEvent{
		{
			Region:  "us-west-2",
			Service: "multi-region-eda",
			Status:  wguevents.StatusHealthy,
			Dependencies: []wguevents.DependencyCheck{
				{Name: "dynamodb", Status: wguevents.StatusHealthy},
			},
			Timestamp: time.Now(),
		},
		{
			Region:  "us-east-1",
			Service: "multi-region-eda",
			Status:  wguevents.StatusUnhealthy,
			Dependencies: []wguevents.DependencyCheck{
				{Name: "dynamodb", Status: wguevents.StatusUnhealthy},
			},
			ErrorMessages: []string{"DynamoDB: unhealthy"},
			Timestamp:     time.Now(),
		},
	}

	aggregated := aggregateHealth(checks)

	assert.Equal(t, wguevents.StatusUnhealthy, aggregated.Status, "Unhealthy status should take precedence")
}

func TestDetermineHealthStatus_PriorityOrder(t *testing.T) {
	// Test that unhealthy > degraded > healthy
	tests := []struct {
		name         string
		dependencies []wguevents.DependencyCheck
		expected     string
	}{
		{
			name: "all healthy",
			dependencies: []wguevents.DependencyCheck{
				{Status: wguevents.StatusHealthy},
				{Status: wguevents.StatusHealthy},
			},
			expected: wguevents.StatusHealthy,
		},
		{
			name: "one degraded",
			dependencies: []wguevents.DependencyCheck{
				{Status: wguevents.StatusHealthy},
				{Status: wguevents.StatusDegraded},
			},
			expected: wguevents.StatusDegraded,
		},
		{
			name: "one unhealthy overrides degraded",
			dependencies: []wguevents.DependencyCheck{
				{Status: wguevents.StatusDegraded},
				{Status: wguevents.StatusUnhealthy},
			},
			expected: wguevents.StatusUnhealthy,
		},
		{
			name: "multiple unhealthy",
			dependencies: []wguevents.DependencyCheck{
				{Status: wguevents.StatusUnhealthy},
				{Status: wguevents.StatusUnhealthy},
			},
			expected: wguevents.StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := determineHealthStatus(tt.dependencies)
			assert.Equal(t, tt.expected, status)
		})
	}
}

func TestCalculateMetrics_VariousLatencies(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Latency: 50 * time.Millisecond, ErrorRate: 0.0},
		{Latency: 100 * time.Millisecond, ErrorRate: 0.0},
		{Latency: 150 * time.Millisecond, ErrorRate: 0.0},
		{Latency: 200 * time.Millisecond, ErrorRate: 0.0},
	}

	metrics := calculateMetrics(dependencies)

	// Average: (50 + 100 + 150 + 200) / 4 = 125ms
	assert.Equal(t, int64(125), metrics.Latency)
}

func TestAggregateHealth_PreservesTimestamp(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC)
	checks := []*wguevents.HealthCheckEvent{
		{
			Region:    "us-west-2",
			Service:   "multi-region-eda",
			Status:    wguevents.StatusHealthy,
			Timestamp: testTime,
		},
	}

	aggregated := aggregateHealth(checks)

	// The timestamp from the first check should be preserved
	assert.Equal(t, testTime, aggregated.Timestamp)
}

func TestAggregateHealth_CombinesErrorMessages(t *testing.T) {
	checks := []*wguevents.HealthCheckEvent{
		{
			Region:        "us-west-2",
			Status:        wguevents.StatusHealthy,
			ErrorMessages: []string{"warning1", "warning2"},
			Timestamp:     time.Now(),
		},
		{
			Region:        "us-east-1",
			Status:        wguevents.StatusDegraded,
			ErrorMessages: []string{"error1"},
			Timestamp:     time.Now(),
		},
	}

	aggregated := aggregateHealth(checks)

	assert.Len(t, aggregated.ErrorMessages, 3)
	assert.Contains(t, aggregated.ErrorMessages, "warning1")
	assert.Contains(t, aggregated.ErrorMessages, "warning2")
	assert.Contains(t, aggregated.ErrorMessages, "error1")
}

func TestCalculateMetrics_HighErrorRates(t *testing.T) {
	dependencies := []wguevents.DependencyCheck{
		{Name: "service1", Latency: 100 * time.Millisecond, ErrorRate: 0.50},
		{Name: "service2", Latency: 200 * time.Millisecond, ErrorRate: 0.75},
	}

	metrics := calculateMetrics(dependencies)

	assert.Equal(t, int64(150), metrics.Latency)
	assert.InDelta(t, 0.625, metrics.ErrorRate, 0.001)
}
