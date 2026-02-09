package awsutils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

const (
	defaultTimeout = 10 * time.Second
	maxBatchSize   = 10 // EventBridge limit
)

// EventBridgePublisher handles publishing events to EventBridge
type EventBridgePublisher struct {
	client    *eventbridge.Client
	eventBus  string
	source    string
	maxRetry  int
	timeout   time.Duration
}

// NewEventBridgePublisher creates a new EventBridge publisher
func NewEventBridgePublisher(client *eventbridge.Client, eventBus, source string) *EventBridgePublisher {
	return &EventBridgePublisher{
		client:   client,
		eventBus: eventBus,
		source:   source,
		maxRetry: 3,
		timeout:  defaultTimeout,
	}
}

// PublishEvent publishes a single event to EventBridge
func (p *EventBridgePublisher) PublishEvent(ctx context.Context, detailType string, detail interface{}) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to marshal event detail: %w", err)
	}

	entry := types.PutEventsRequestEntry{
		EventBusName: aws.String(p.eventBus),
		Source:       aws.String(p.source),
		DetailType:   aws.String(detailType),
		Detail:       aws.String(string(detailJSON)),
		Time:         aws.Time(time.Now()),
	}

	return p.publishEntries(ctx, []types.PutEventsRequestEntry{entry})
}

// PublishEventBatch publishes multiple events in a batch
func (p *EventBridgePublisher) PublishEventBatch(ctx context.Context, events []EventBridgeEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Split into batches of maxBatchSize
	for i := 0; i < len(events); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		entries := make([]types.PutEventsRequestEntry, len(batch))

		for j, event := range batch {
			detailJSON, err := json.Marshal(event.Detail)
			if err != nil {
				return fmt.Errorf("failed to marshal event detail at index %d: %w", j, err)
			}

			entries[j] = types.PutEventsRequestEntry{
				EventBusName: aws.String(p.eventBus),
				Source:       aws.String(p.source),
				DetailType:   aws.String(event.DetailType),
				Detail:       aws.String(string(detailJSON)),
				Time:         aws.Time(time.Now()),
			}
		}

		if err := p.publishEntries(ctx, entries); err != nil {
			return fmt.Errorf("failed to publish batch starting at index %d: %w", i, err)
		}
	}

	return nil
}

// publishEntries publishes entries with retry logic
func (p *EventBridgePublisher) publishEntries(ctx context.Context, entries []types.PutEventsRequestEntry) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= p.maxRetry; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			time.Sleep(backoff)
		}

		output, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
			Entries: entries,
		})

		if err != nil {
			lastErr = err
			continue
		}

		// Check for failed entries
		if output.FailedEntryCount > 0 {
			failedEntries := make([]types.PutEventsRequestEntry, 0)
			for i, entry := range output.Entries {
				if entry.ErrorCode != nil {
					failedEntries = append(failedEntries, entries[i])
					lastErr = fmt.Errorf("entry failed with code %s: %s", 
						aws.ToString(entry.ErrorCode), 
						aws.ToString(entry.ErrorMessage))
				}
			}

			// Retry only failed entries
			if len(failedEntries) > 0 {
				entries = failedEntries
				continue
			}
		}

		// Success
		return nil
	}

	return fmt.Errorf("failed to publish events after %d attempts: %w", p.maxRetry, lastErr)
}

// EventBridgeEvent represents an event to be published
type EventBridgeEvent struct {
	DetailType string
	Detail     interface{}
}

// PublishCrossRegionEvent publishes an event to a partner region's EventBridge
func (p *EventBridgePublisher) PublishCrossRegionEvent(ctx context.Context, targetRegion string, event interface{}) error {
	detailType := fmt.Sprintf("cross-region.%s", targetRegion)
	return p.PublishEvent(ctx, detailType, event)
}
