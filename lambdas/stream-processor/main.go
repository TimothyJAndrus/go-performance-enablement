package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wgu/go-performance-enablement/pkg/awsutils"
	wguevents "github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

var (
	logger         *zap.Logger
	awsClients     *awsutils.AWSClients
	publisher      *awsutils.EventBridgePublisher
	dynamoHelper   *awsutils.DynamoDBHelper
	currentRegion  string
	eventBusName   string
	replicaTable   string
	dlqURL         string
)

func init() {
	var err error
	
	// Initialize logger
	logger, _ = zap.NewProduction()
	
	// Get environment variables
	currentRegion = os.Getenv("AWS_REGION")
	eventBusName = os.Getenv("EVENT_BUS_NAME")
	replicaTable = os.Getenv("REPLICA_TABLE_NAME")
	dlqURL = os.Getenv("DLQ_URL")
	
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
		"stream-processor",
	)
	
	// Initialize DynamoDB helper
	dynamoHelper = awsutils.NewDynamoDBHelper(awsClients.DynamoDB, replicaTable)
}

// Handler processes DynamoDB Stream events
func Handler(ctx context.Context, event events.DynamoDBEvent) error {
	start := time.Now()
	functionName := "stream-processor"
	
	logger.Info("processing DynamoDB stream batch",
		zap.Int("record_count", len(event.Records)),
		zap.String("region", currentRegion),
	)
	
	var errors []error
	
	for _, record := range event.Records {
		if err := processStreamRecord(ctx, record); err != nil {
			errors = append(errors, err)
			logger.Error("failed to process stream record",
				zap.Error(err),
				zap.String("event_id", record.EventID),
				zap.String("event_name", record.EventName),
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
	
	logger.Info("successfully processed stream batch",
		zap.Duration("duration", duration),
		zap.Int("record_count", len(event.Records)),
	)
	
	return nil
}

func processStreamRecord(ctx context.Context, record events.DynamoDBEventRecord) error {
	start := time.Now()
	
	// Convert to CDC event
	cdcEvent, err := toCDCEvent(record)
	if err != nil {
		return fmt.Errorf("failed to convert to CDC event: %w", err)
	}
	
	// Process based on operation type
	var processingErr error
	switch cdcEvent.Operation {
	case wguevents.OperationInsert:
		processingErr = handleInsert(ctx, cdcEvent)
	case wguevents.OperationUpdate:
		processingErr = handleUpdate(ctx, cdcEvent)
	case wguevents.OperationDelete:
		processingErr = handleDelete(ctx, cdcEvent)
	default:
		processingErr = fmt.Errorf("unknown operation: %s", cdcEvent.Operation)
	}
	
	if processingErr != nil {
		// Send to DLQ
		if dlqErr := sendToDLQ(ctx, cdcEvent, processingErr); dlqErr != nil {
			logger.Error("failed to send to DLQ",
				zap.Error(dlqErr),
				zap.String("event_id", record.EventID),
			)
		}
		return processingErr
	}
	
	// Publish event to EventBridge
	baseEvent := wguevents.NewBaseEvent(
		fmt.Sprintf("cdc.%s", cdcEvent.Operation),
		currentRegion,
		map[string]interface{}{
			"table":      cdcEvent.TableName,
			"operation":  cdcEvent.Operation,
			"after":      cdcEvent.After,
			"before":     cdcEvent.Before,
			"primaryKeys": cdcEvent.PrimaryKeys,
		},
	)
	
	if err := publisher.PublishEvent(ctx, baseEvent.EventType, baseEvent); err != nil {
		logger.Error("failed to publish event",
			zap.Error(err),
			zap.String("event_type", baseEvent.EventType),
		)
		// Don't fail the Lambda on EventBridge errors
	}
	
	// Record metrics
	duration := time.Since(start)
	metrics.RecordCDCEvent(cdcEvent.Operation, cdcEvent.TableName, "dynamodb-streams", duration)
	
	logger.Debug("processed CDC event",
		zap.String("operation", cdcEvent.Operation),
		zap.String("table", cdcEvent.TableName),
		zap.Duration("duration", duration),
	)
	
	return nil
}

func toCDCEvent(record events.DynamoDBEventRecord) (*wguevents.CDCEvent, error) {
	var operation string
	switch record.EventName {
	case "INSERT":
		operation = wguevents.OperationInsert
	case "MODIFY":
		operation = wguevents.OperationUpdate
	case "REMOVE":
		operation = wguevents.OperationDelete
	default:
		return nil, fmt.Errorf("unknown event name: %s", record.EventName)
	}
	
	cdcEvent := &wguevents.CDCEvent{
		Operation:     operation,
		TableName:     extractTableName(record.EventSourceArn),
		Timestamp:     record.Change.ApproximateCreationDateTime.Time,
		PrimaryKeys:   convertAttributeValues(record.Change.Keys),
		After:         convertAttributeValues(record.Change.NewImage),
		Before:        convertAttributeValues(record.Change.OldImage),
		Metadata: wguevents.CDCMetadata{
			SourceDatabase: "dynamodb",
			SourceTable:    extractTableName(record.EventSourceArn),
			Offset:         0,
			Partition:      0,
			CaptureTime:    record.Change.ApproximateCreationDateTime.Time,
		},
	}
	
	return cdcEvent, nil
}

func extractTableName(arn string) string {
	// Parse ARN to extract table name
	// ARN format: arn:aws:dynamodb:region:account:table/TableName/stream/timestamp
	// Simple implementation - could use AWS SDK ARN parser
	return "events" // placeholder
}

func convertAttributeValues(attrs map[string]events.DynamoDBAttributeValue) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range attrs {
		// Convert DynamoDB attribute value to generic interface{}
		// This is a simplified conversion
		if value.String() != "" {
			result[key] = value.String()
		} else if value.Number() != "" {
			result[key] = value.Number()
		} else if value.Boolean() {
			result[key] = value.Boolean()
		}
		// Add more type conversions as needed
	}
	return result
}

func handleInsert(ctx context.Context, event *wguevents.CDCEvent) error {
	logger.Debug("handling INSERT operation",
		zap.String("table", event.TableName),
		zap.Any("data", event.After),
	)
	
	// Replicate to partner region table
	if replicaTable != "" {
		if err := dynamoHelper.PutItem(ctx, event.After); err != nil {
			return fmt.Errorf("failed to replicate INSERT: %w", err)
		}
	}
	
	metrics.DynamoDBOperations.WithLabelValues(event.TableName, "INSERT", currentRegion).Inc()
	return nil
}

func handleUpdate(ctx context.Context, event *wguevents.CDCEvent) error {
	logger.Debug("handling UPDATE operation",
		zap.String("table", event.TableName),
		zap.Any("before", event.Before),
		zap.Any("after", event.After),
	)
	
	// Replicate to partner region table
	if replicaTable != "" {
		if err := dynamoHelper.PutItem(ctx, event.After); err != nil {
			return fmt.Errorf("failed to replicate UPDATE: %w", err)
		}
	}
	
	metrics.DynamoDBOperations.WithLabelValues(event.TableName, "UPDATE", currentRegion).Inc()
	return nil
}

func handleDelete(ctx context.Context, event *wguevents.CDCEvent) error {
	logger.Debug("handling DELETE operation",
		zap.String("table", event.TableName),
		zap.Any("primaryKeys", event.PrimaryKeys),
	)
	
	// Replicate delete to partner region table
	if replicaTable != "" {
		// Convert primary keys to DynamoDB attribute values
		// This is simplified - real implementation would need proper type conversion
		// if err := dynamoHelper.DeleteItem(ctx, event.PrimaryKeys); err != nil {
		// 	return fmt.Errorf("failed to replicate DELETE: %w", err)
		// }
	}
	
	metrics.DynamoDBOperations.WithLabelValues(event.TableName, "DELETE", currentRegion).Inc()
	return nil
}

func sendToDLQ(ctx context.Context, event *wguevents.CDCEvent, processingError error) error {
	dlqEvent := &wguevents.DeadLetterEvent{
		ErrorMessage:  processingError.Error(),
		ErrorType:     "cdc_processing_failure",
		FailureCount:  1,
		FirstFailure:  time.Now(),
		LastFailure:   time.Now(),
		SourceHandler: "stream-processor",
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
	
	metrics.DLQMessages.WithLabelValues("stream-processor", "cdc_processing_failure").Inc()
	
	return nil
}

func main() {
	lambda.Start(Handler)
}
