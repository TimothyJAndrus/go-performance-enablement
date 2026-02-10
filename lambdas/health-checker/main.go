package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wgu/go-performance-enablement/pkg/awsutils"
	wguevents "github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

var (
	logger         *zap.Logger
	awsClients     *awsutils.AWSClients
	partnerClients *awsutils.AWSClients
	publisher      *awsutils.EventBridgePublisher
	currentRegion  string
	partnerRegion  string
	eventBusName   string
)

func init() {
	var err error

	// Initialize logger
	logger, _ = zap.NewProduction()

	// Get environment variables
	currentRegion = os.Getenv("AWS_REGION")
	partnerRegion = os.Getenv("PARTNER_REGION")
	eventBusName = os.Getenv("EVENT_BUS_NAME")

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
		awsClients.EventBridge,
		eventBusName,
		"health-checker",
	)
}

// HealthCheckRequest represents a scheduled health check request
type HealthCheckRequest struct {
	CheckType string `json:"check_type"` // full, quick
}

// Handler performs health checks across regions
func Handler(ctx context.Context, request HealthCheckRequest) error {
	start := time.Now()
	functionName := "health-checker"

	logger.Info("starting health check",
		zap.String("check_type", request.CheckType),
		zap.String("current_region", currentRegion),
		zap.String("partner_region", partnerRegion),
	)

	// Perform health checks in parallel
	var wg sync.WaitGroup
	healthChecks := make(chan *wguevents.HealthCheckEvent, 2)
	errors := make(chan error, 2)

	// Check current region
	wg.Add(1)
	go func() {
		defer wg.Done()
		health, err := checkRegionHealth(ctx, currentRegion, awsClients)
		if err != nil {
			errors <- fmt.Errorf("failed to check current region: %w", err)
			return
		}
		healthChecks <- health
	}()

	// Check partner region
	wg.Add(1)
	go func() {
		defer wg.Done()
		health, err := checkRegionHealth(ctx, partnerRegion, partnerClients)
		if err != nil {
			errors <- fmt.Errorf("failed to check partner region: %w", err)
			return
		}
		healthChecks <- health
	}()

	// Wait for checks to complete
	wg.Wait()
	close(healthChecks)
	close(errors)

	// Collect results
	var results []*wguevents.HealthCheckEvent
	for health := range healthChecks {
		results = append(results, health)
	}

	// Check for errors
	var checkErrors []error
	for err := range errors {
		checkErrors = append(checkErrors, err)
		logger.Error("health check error", zap.Error(err))
	}

	// Aggregate health status
	aggregatedHealth := aggregateHealth(results)

	// Publish health check results
	if err := publisher.PublishEvent(ctx, wguevents.EventTypeHealthCheck, aggregatedHealth); err != nil {
		logger.Error("failed to publish health check", zap.Error(err))
	}

	// Log summary
	duration := time.Since(start)
	metrics.RecordLambdaInvocation(functionName, currentRegion, duration, nil)

	logger.Info("health check complete",
		zap.Duration("duration", duration),
		zap.String("overall_status", aggregatedHealth.Status),
		zap.Int("regions_checked", len(results)),
		zap.Int("errors", len(checkErrors)),
	)

	if len(checkErrors) > 0 {
		return fmt.Errorf("health check completed with %d errors", len(checkErrors))
	}

	return nil
}

// checkRegionHealth performs health checks for a specific region
func checkRegionHealth(ctx context.Context, region string, clients *awsutils.AWSClients) (*wguevents.HealthCheckEvent, error) {
	logger.Info("checking region health", zap.String("region", region))

	health := &wguevents.HealthCheckEvent{
		Region:    region,
		Service:   "multi-region-eda",
		Timestamp: time.Now(),
		Dependencies: []wguevents.DependencyCheck{},
		Metrics: wguevents.HealthMetrics{},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errorMessages := []string{}

	// Check DynamoDB
	wg.Add(1)
	go func() {
		defer wg.Done()
		dep := checkDynamoDB(ctx, clients)
		mu.Lock()
		health.Dependencies = append(health.Dependencies, dep)
		if dep.Status != wguevents.StatusHealthy {
			errorMessages = append(errorMessages, fmt.Sprintf("DynamoDB: %s", dep.Status))
		}
		mu.Unlock()
	}()

	// Check EventBridge
	wg.Add(1)
	go func() {
		defer wg.Done()
		dep := checkEventBridge(ctx, clients)
		mu.Lock()
		health.Dependencies = append(health.Dependencies, dep)
		if dep.Status != wguevents.StatusHealthy {
			errorMessages = append(errorMessages, fmt.Sprintf("EventBridge: %s", dep.Status))
		}
		mu.Unlock()
	}()

	// Check SQS
	wg.Add(1)
	go func() {
		defer wg.Done()
		dep := checkSQS(ctx, clients)
		mu.Lock()
		health.Dependencies = append(health.Dependencies, dep)
		if dep.Status != wguevents.StatusHealthy {
			errorMessages = append(errorMessages, fmt.Sprintf("SQS: %s", dep.Status))
		}
		mu.Unlock()
	}()

	wg.Wait()

	// Determine overall status
	health.Status = determineHealthStatus(health.Dependencies)
	health.ErrorMessages = errorMessages

	// Calculate aggregate metrics
	health.Metrics = calculateMetrics(health.Dependencies)

	return health, nil
}

// checkDynamoDB checks DynamoDB health
func checkDynamoDB(ctx context.Context, clients *awsutils.AWSClients) wguevents.DependencyCheck {
	start := time.Now()

	// Simple health check - list tables with limit
	_, err := clients.DynamoDB.ListTables(ctx, nil)
	latency := time.Since(start)

	status := wguevents.StatusHealthy
	if err != nil {
		status = wguevents.StatusUnhealthy
		logger.Error("DynamoDB health check failed", zap.Error(err))
	} else if latency > 500*time.Millisecond {
		status = wguevents.StatusDegraded
	}

	return wguevents.DependencyCheck{
		Name:      "dynamodb",
		Type:      "database",
		Status:    status,
		Latency:   latency,
		ErrorRate: 0.0,
	}
}

// checkEventBridge checks EventBridge health
func checkEventBridge(ctx context.Context, clients *awsutils.AWSClients) wguevents.DependencyCheck {
	start := time.Now()

	// Simple health check - list event buses
	_, err := clients.EventBridge.ListEventBuses(ctx, nil)
	latency := time.Since(start)

	status := wguevents.StatusHealthy
	if err != nil {
		status = wguevents.StatusUnhealthy
		logger.Error("EventBridge health check failed", zap.Error(err))
	} else if latency > 500*time.Millisecond {
		status = wguevents.StatusDegraded
	}

	return wguevents.DependencyCheck{
		Name:      "eventbridge",
		Type:      "api",
		Status:    status,
		Latency:   latency,
		ErrorRate: 0.0,
	}
}

// checkSQS checks SQS health
func checkSQS(ctx context.Context, clients *awsutils.AWSClients) wguevents.DependencyCheck {
	start := time.Now()

	// Simple health check - list queues
	_, err := clients.SQS.ListQueues(ctx, nil)
	latency := time.Since(start)

	status := wguevents.StatusHealthy
	if err != nil {
		status = wguevents.StatusUnhealthy
		logger.Error("SQS health check failed", zap.Error(err))
	} else if latency > 500*time.Millisecond {
		status = wguevents.StatusDegraded
	}

	return wguevents.DependencyCheck{
		Name:      "sqs",
		Type:      "api",
		Status:    status,
		Latency:   latency,
		ErrorRate: 0.0,
	}
}

// determineHealthStatus determines overall health from dependencies
func determineHealthStatus(dependencies []wguevents.DependencyCheck) string {
	hasUnhealthy := false
	hasDegraded := false

	for _, dep := range dependencies {
		switch dep.Status {
		case wguevents.StatusUnhealthy:
			hasUnhealthy = true
		case wguevents.StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return wguevents.StatusUnhealthy
	}
	if hasDegraded {
		return wguevents.StatusDegraded
	}
	return wguevents.StatusHealthy
}

// calculateMetrics calculates aggregate metrics from dependencies
func calculateMetrics(dependencies []wguevents.DependencyCheck) wguevents.HealthMetrics {
	var totalLatency time.Duration
	var totalErrorRate float64

	for _, dep := range dependencies {
		totalLatency += dep.Latency
		totalErrorRate += dep.ErrorRate
	}

	count := len(dependencies)
	avgLatency := int64(0)
	avgErrorRate := 0.0

	if count > 0 {
		avgLatency = totalLatency.Milliseconds() / int64(count)
		avgErrorRate = totalErrorRate / float64(count)
	}

	return wguevents.HealthMetrics{
		Latency:   avgLatency,
		ErrorRate: avgErrorRate,
	}
}

// aggregateHealth aggregates health from multiple regions
func aggregateHealth(checks []*wguevents.HealthCheckEvent) *wguevents.HealthCheckEvent {
	if len(checks) == 0 {
		return &wguevents.HealthCheckEvent{
			Status:    wguevents.StatusUnhealthy,
			Timestamp: time.Now(),
			Service:   "multi-region-eda",
		}
	}

	// Use first check as base
	aggregated := checks[0]
	aggregated.Region = "multi-region"

	// Aggregate dependencies from all regions
	allDeps := []wguevents.DependencyCheck{}
	for _, check := range checks {
		allDeps = append(allDeps, check.Dependencies...)
	}
	aggregated.Dependencies = allDeps

	// Determine worst status
	aggregated.Status = determineHealthStatus(allDeps)
	aggregated.Metrics = calculateMetrics(allDeps)

	// Collect all error messages
	allErrors := []string{}
	for _, check := range checks {
		allErrors = append(allErrors, check.ErrorMessages...)
	}
	aggregated.ErrorMessages = allErrors

	return aggregated
}

func main() {
	lambda.Start(Handler)
}
