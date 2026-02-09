package consumer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

// KafkaConfig holds Kafka consumer configuration
type KafkaConfig struct {
	BootstrapServers string
	GroupID          string
	Topics           []string
	SecurityProtocol string
	SASLMechanism    string
	SASLUsername     string
	SASLPassword     string
	SchemaRegistry   string
	AutoOffsetReset  string
}

// MessageProcessor defines the interface for processing Kafka messages
type MessageProcessor interface {
	Process(ctx context.Context, msg *kafka.Message) error
}

// KafkaConsumer wraps Confluent Kafka consumer
type KafkaConsumer struct {
	consumer *kafka.Consumer
	topics   []string
	logger   *zap.Logger
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(config *KafkaConfig, logger *zap.Logger) (*KafkaConsumer, error) {
	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers":  config.BootstrapServers,
		"group.id":           config.GroupID,
		"auto.offset.reset":  config.AutoOffsetReset,
		"enable.auto.commit": false, // Manual offset commit for better control
		"session.timeout.ms": 30000,
		"max.poll.interval.ms": 300000,
	}

	// Add security configuration if needed
	if config.SecurityProtocol != "PLAINTEXT" {
		kafkaConfig.SetKey("security.protocol", config.SecurityProtocol)
		kafkaConfig.SetKey("sasl.mechanism", config.SASLMechanism)
		kafkaConfig.SetKey("sasl.username", config.SASLUsername)
		kafkaConfig.SetKey("sasl.password", config.SASLPassword)
	}

	consumer, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	logger.Info("created Kafka consumer",
		zap.String("bootstrap_servers", config.BootstrapServers),
		zap.String("group_id", config.GroupID),
		zap.Strings("topics", config.Topics),
	)

	return &KafkaConsumer{
		consumer: consumer,
		topics:   config.Topics,
		logger:   logger,
	}, nil
}

// Consume starts consuming messages from Kafka
func (kc *KafkaConsumer) Consume(ctx context.Context, processor MessageProcessor) error {
	// Subscribe to topics
	if err := kc.consumer.SubscribeTopics(kc.topics, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	kc.logger.Info("subscribed to topics", zap.Strings("topics", kc.topics))

	// Start consuming loop
	for {
		select {
		case <-ctx.Done():
			kc.logger.Info("stopping consumer due to context cancellation")
			return ctx.Err()
		default:
			if err := kc.consumeMessage(ctx, processor); err != nil {
				kc.logger.Error("error consuming message", zap.Error(err))
				// Continue consuming on error
			}
		}
	}
}

// consumeMessage consumes and processes a single message
func (kc *KafkaConsumer) consumeMessage(ctx context.Context, processor MessageProcessor) error {
	start := time.Now()

	// Poll for message with timeout
	msg, err := kc.consumer.ReadMessage(1 * time.Second)
	if err != nil {
		// Timeout is not an error, just no messages available
		if err.(kafka.Error).Code() == kafka.ErrTimedOut {
			return nil
		}
		return fmt.Errorf("failed to read message: %w", err)
	}

	topic := *msg.TopicPartition.Topic
	partition := strconv.Itoa(int(msg.TopicPartition.Partition))

	kc.logger.Debug("received message",
		zap.String("topic", topic),
		zap.String("partition", partition),
		zap.Int64("offset", int64(msg.TopicPartition.Offset)),
	)

	// Process the message
	processingStart := time.Now()
	err = processor.Process(ctx, msg)
	processingDuration := time.Since(processingStart)

	if err != nil {
		kc.logger.Error("failed to process message",
			zap.Error(err),
			zap.String("topic", topic),
			zap.String("partition", partition),
			zap.Int64("offset", int64(msg.TopicPartition.Offset)),
		)
		
		metrics.RecordKafkaMessage(topic, partition, kc.consumer.GetMetadata().OriginatingBrokerName, processingDuration, err)
		
		// Don't commit offset on error - message will be reprocessed
		return err
	}

	// Commit offset after successful processing
	if _, err := kc.consumer.CommitMessage(msg); err != nil {
		kc.logger.Error("failed to commit offset",
			zap.Error(err),
			zap.String("topic", topic),
			zap.Int64("offset", int64(msg.TopicPartition.Offset)),
		)
		return fmt.Errorf("failed to commit offset: %w", err)
	}

	// Record metrics
	totalDuration := time.Since(start)
	metrics.RecordKafkaMessage(topic, partition, "go-cdc-consumers", processingDuration, nil)

	// Calculate and record consumer lag
	if msg.Timestamp.Valid {
		lag := time.Since(msg.Timestamp.Time)
		metrics.KafkaConsumerLag.WithLabelValues(topic, partition, "go-cdc-consumers").Set(lag.Seconds())
	}

	kc.logger.Debug("successfully processed message",
		zap.String("topic", topic),
		zap.Duration("processing_duration", processingDuration),
		zap.Duration("total_duration", totalDuration),
	)

	return nil
}

// Close closes the Kafka consumer
func (kc *KafkaConsumer) Close() error {
	kc.logger.Info("closing Kafka consumer")
	return kc.consumer.Close()
}

// GetConsumerGroupMetadata returns consumer group metadata
func (kc *KafkaConsumer) GetConsumerGroupMetadata() (*kafka.ConsumerGroupMetadata, error) {
	return kc.consumer.GetConsumerGroupMetadata()
}
