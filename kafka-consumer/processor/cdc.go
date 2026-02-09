package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/linkedin/goavro/v2"
	"github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

// CDCProcessor processes CDC events from Kafka
type CDCProcessor struct {
	logger *zap.Logger
	codec  *goavro.Codec
}

// NewCDCProcessor creates a new CDC processor
func NewCDCProcessor(logger *zap.Logger) *CDCProcessor {
	return &CDCProcessor{
		logger: logger,
	}
}

// Process processes a Kafka message containing a CDC event
func (p *CDCProcessor) Process(ctx context.Context, msg *kafka.Message) error {
	start := time.Now()

	// Parse CDC event from message
	cdcEvent, err := p.parseCDCEvent(msg)
	if err != nil {
		return fmt.Errorf("failed to parse CDC event: %w", err)
	}

	// Process based on operation type
	switch cdcEvent.Operation {
	case events.OperationInsert:
		err = p.handleInsert(ctx, cdcEvent)
	case events.OperationUpdate:
		err = p.handleUpdate(ctx, cdcEvent)
	case events.OperationDelete:
		err = p.handleDelete(ctx, cdcEvent)
	case events.OperationRefresh:
		err = p.handleRefresh(ctx, cdcEvent)
	default:
		err = fmt.Errorf("unknown operation: %s", cdcEvent.Operation)
	}

	if err != nil {
		return fmt.Errorf("failed to process CDC event: %w", err)
	}

	// Record metrics
	duration := time.Since(start)
	metrics.RecordCDCEvent(cdcEvent.Operation, cdcEvent.TableName, "qlik", duration)

	p.logger.Debug("processed CDC event",
		zap.String("operation", cdcEvent.Operation),
		zap.String("table", cdcEvent.TableName),
		zap.Duration("duration", duration),
	)

	return nil
}

// parseCDCEvent parses a CDC event from a Kafka message
func (p *CDCProcessor) parseCDCEvent(msg *kafka.Message) (*events.CDCEvent, error) {
	var cdcEvent events.CDCEvent

	// Try JSON first (for local development)
	if err := json.Unmarshal(msg.Value, &cdcEvent); err == nil {
		return &cdcEvent, nil
	}

	// If JSON fails, try Avro deserialization
	if p.codec != nil {
		native, _, err := p.codec.NativeFromBinary(msg.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize Avro: %w", err)
		}

		// Convert native to CDCEvent
		jsonBytes, err := json.Marshal(native)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal native to JSON: %w", err)
		}

		if err := json.Unmarshal(jsonBytes, &cdcEvent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON to CDCEvent: %w", err)
		}

		return &cdcEvent, nil
	}

	return nil, fmt.Errorf("failed to parse CDC event: unsupported format")
}

// handleInsert processes an INSERT operation
func (p *CDCProcessor) handleInsert(ctx context.Context, event *events.CDCEvent) error {
	p.logger.Info("handling INSERT",
		zap.String("table", event.TableName),
		zap.Any("after", event.After),
	)

	// Business logic for INSERT
	// - Store in database
	// - Update cache
	// - Publish domain event
	// - etc.

	return nil
}

// handleUpdate processes an UPDATE operation
func (p *CDCProcessor) handleUpdate(ctx context.Context, event *events.CDCEvent) error {
	p.logger.Info("handling UPDATE",
		zap.String("table", event.TableName),
		zap.Any("before", event.Before),
		zap.Any("after", event.After),
	)

	// Business logic for UPDATE
	// - Update in database
	// - Invalidate cache
	// - Publish domain event
	// - etc.

	return nil
}

// handleDelete processes a DELETE operation
func (p *CDCProcessor) handleDelete(ctx context.Context, event *events.CDCEvent) error {
	p.logger.Info("handling DELETE",
		zap.String("table", event.TableName),
		zap.Any("before", event.Before),
	)

	// Business logic for DELETE
	// - Delete from database
	// - Remove from cache
	// - Publish domain event
	// - etc.

	return nil
}

// handleRefresh processes a REFRESH operation
func (p *CDCProcessor) handleRefresh(ctx context.Context, event *events.CDCEvent) error {
	p.logger.Info("handling REFRESH",
		zap.String("table", event.TableName),
		zap.Any("after", event.After),
	)

	// Business logic for REFRESH (full load)
	// - Truncate and reload data
	// - Clear all caches
	// - etc.

	return nil
}

// SetAvroCodec sets the Avro codec for deserialization
func (p *CDCProcessor) SetAvroCodec(codec *goavro.Codec) {
	p.codec = codec
}
