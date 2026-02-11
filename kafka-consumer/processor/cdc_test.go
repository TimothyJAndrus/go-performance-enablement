package processor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/wgu/go-performance-enablement/pkg/events"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewCDCProcessor(t *testing.T) {
	logger, _ := zap.NewProduction()
	
	processor := NewCDCProcessor(logger)
	
	assert.NotNil(t, processor)
	assert.NotNil(t, processor.logger)
	assert.Nil(t, processor.codec)
}

func TestSetAvroCodec(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	
	// Codec is initially nil
	assert.Nil(t, processor.codec)
	
	// Setting codec should work (we can't create real codec without schema)
	// Just test that SetAvroCodec doesn't panic
	assert.NotPanics(t, func() {
		processor.SetAvroCodec(nil)
	})
}

func TestParseCDCEvent_ValidJSON(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	
	cdcEvent := &events.CDCEvent{
		Operation: events.OperationInsert,
		TableName: "customers",
		Timestamp: time.Now(),
		After: map[string]interface{}{
			"id":   "cust-123",
			"name": "John Doe",
		},
		Metadata: events.CDCMetadata{
			SourceDatabase: "qlik",
			SourceTable:    "customers",
		},
	}
	
	jsonBytes, err := json.Marshal(cdcEvent)
	assert.NoError(t, err)
	
	msg := &kafka.Message{
		Value: jsonBytes,
	}
	
	parsed, err := processor.parseCDCEvent(msg)
	
	assert.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, events.OperationInsert, parsed.Operation)
	assert.Equal(t, "customers", parsed.TableName)
}

func TestParseCDCEvent_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	
	msg := &kafka.Message{
		Value: []byte("invalid json"),
	}
	
	parsed, err := processor.parseCDCEvent(msg)
	
	assert.Error(t, err)
	assert.Nil(t, parsed)
}

func TestHandleInsert(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	event := &events.CDCEvent{
		Operation: events.OperationInsert,
		TableName: "customers",
		After: map[string]interface{}{
			"id":   "cust-123",
			"name": "John Doe",
		},
	}
	
	err := processor.handleInsert(ctx, event)
	
	// Basic implementation returns nil
	assert.NoError(t, err)
}

func TestHandleUpdate(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	event := &events.CDCEvent{
		Operation: events.OperationUpdate,
		TableName: "customers",
		Before: map[string]interface{}{
			"id":   "cust-123",
			"name": "John Doe",
		},
		After: map[string]interface{}{
			"id":   "cust-123",
			"name": "Jane Doe",
		},
	}
	
	err := processor.handleUpdate(ctx, event)
	
	// Basic implementation returns nil
	assert.NoError(t, err)
}

func TestHandleDelete(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	event := &events.CDCEvent{
		Operation: events.OperationDelete,
		TableName: "customers",
		Before: map[string]interface{}{
			"id":   "cust-123",
			"name": "John Doe",
		},
	}
	
	err := processor.handleDelete(ctx, event)
	
	// Basic implementation returns nil
	assert.NoError(t, err)
}

func TestHandleRefresh(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	event := &events.CDCEvent{
		Operation: events.OperationRefresh,
		TableName: "customers",
		After: map[string]interface{}{
			"id":   "cust-123",
			"name": "John Doe",
		},
	}
	
	err := processor.handleRefresh(ctx, event)
	
	// Basic implementation returns nil
	assert.NoError(t, err)
}

func TestProcess_ValidInsert(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	cdcEvent := &events.CDCEvent{
		Operation: events.OperationInsert,
		TableName: "customers",
		Timestamp: time.Now(),
		After: map[string]interface{}{
			"id":   "cust-123",
			"name": "John Doe",
		},
		Metadata: events.CDCMetadata{
			SourceDatabase: "qlik",
			SourceTable:    "customers",
		},
	}
	
	jsonBytes, err := json.Marshal(cdcEvent)
	assert.NoError(t, err)
	
	msg := &kafka.Message{
		Value: jsonBytes,
	}
	
	err = processor.Process(ctx, msg)
	
	assert.NoError(t, err)
}

func TestProcess_ValidUpdate(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	cdcEvent := &events.CDCEvent{
		Operation: events.OperationUpdate,
		TableName: "orders",
		Timestamp: time.Now(),
		Before: map[string]interface{}{
			"id":     "order-456",
			"status": "pending",
		},
		After: map[string]interface{}{
			"id":     "order-456",
			"status": "shipped",
		},
		Metadata: events.CDCMetadata{
			SourceDatabase: "qlik",
			SourceTable:    "orders",
		},
	}
	
	jsonBytes, err := json.Marshal(cdcEvent)
	assert.NoError(t, err)
	
	msg := &kafka.Message{
		Value: jsonBytes,
	}
	
	err = processor.Process(ctx, msg)
	
	assert.NoError(t, err)
}

func TestProcess_ValidDelete(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	cdcEvent := &events.CDCEvent{
		Operation: events.OperationDelete,
		TableName: "customers",
		Timestamp: time.Now(),
		Before: map[string]interface{}{
			"id":   "cust-789",
			"name": "Test User",
		},
		Metadata: events.CDCMetadata{
			SourceDatabase: "qlik",
			SourceTable:    "customers",
		},
	}
	
	jsonBytes, err := json.Marshal(cdcEvent)
	assert.NoError(t, err)
	
	msg := &kafka.Message{
		Value: jsonBytes,
	}
	
	err = processor.Process(ctx, msg)
	
	assert.NoError(t, err)
}

func TestProcess_InvalidMessage(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	msg := &kafka.Message{
		Value: []byte("invalid json"),
	}
	
	err := processor.Process(ctx, msg)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse CDC event")
}

func TestProcess_UnknownOperation(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	ctx := context.Background()
	
	cdcEvent := &events.CDCEvent{
		Operation: "UNKNOWN_OP",
		TableName: "customers",
		Timestamp: time.Now(),
	}
	
	jsonBytes, err := json.Marshal(cdcEvent)
	assert.NoError(t, err)
	
	msg := &kafka.Message{
		Value: jsonBytes,
	}
	
	err = processor.Process(ctx, msg)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestParseCDCEvent_AllOperations(t *testing.T) {
	logger, _ := zap.NewProduction()
	processor := NewCDCProcessor(logger)
	
	operations := []string{
		events.OperationInsert,
		events.OperationUpdate,
		events.OperationDelete,
		events.OperationRefresh,
	}
	
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			cdcEvent := &events.CDCEvent{
				Operation: op,
				TableName: "test_table",
				Timestamp: time.Now(),
			}
			
			jsonBytes, err := json.Marshal(cdcEvent)
			assert.NoError(t, err)
			
			msg := &kafka.Message{
				Value: jsonBytes,
			}
			
			parsed, err := processor.parseCDCEvent(msg)
			
			assert.NoError(t, err)
			assert.NotNil(t, parsed)
			assert.Equal(t, op, parsed.Operation)
		})
	}
}
