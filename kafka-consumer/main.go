package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wgu/go-performance-enablement/kafka-consumer/consumer"
	"github.com/wgu/go-performance-enablement/kafka-consumer/processor"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

const (
	defaultMetricsPort = ":9090"
	shutdownTimeout    = 30 * time.Second
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting Kafka CDC consumer")

	// Load configuration from environment
	config := loadConfig()

	// Start metrics server
	metricsServer := metrics.NewMetricsServer(config.MetricsPort)
	go func() {
		logger.Info("starting metrics server", zap.String("port", config.MetricsPort))
		if err := metricsServer.Start(); err != nil {
			logger.Error("metrics server failed", zap.Error(err))
		}
	}()

	// Create Kafka consumer
	kafkaConsumer, err := consumer.NewKafkaConsumer(config.KafkaConfig, logger)
	if err != nil {
		logger.Fatal("failed to create Kafka consumer", zap.Error(err))
	}
	defer kafkaConsumer.Close()

	// Create CDC processor
	cdcProcessor := processor.NewCDCProcessor(logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming messages
	go func() {
		if err := kafkaConsumer.Consume(ctx, cdcProcessor); err != nil {
			logger.Error("consumer error", zap.Error(err))
			cancel()
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("received shutdown signal", zap.String("signal", sig.String()))

	// Initiate graceful shutdown
	cancel()

	// Shutdown metrics server
	_, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownTimeout); err != nil {
		logger.Error("failed to shutdown metrics server", zap.Error(err))
	}

	logger.Info("shutdown complete", zap.Duration("timeout", shutdownTimeout))
}

// Config holds application configuration
type Config struct {
	KafkaConfig *consumer.KafkaConfig
	MetricsPort string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		KafkaConfig: &consumer.KafkaConfig{
			BootstrapServers: getEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092"),
			GroupID:          getEnv("KAFKA_GROUP_ID", "go-cdc-consumers"),
			Topics:           getEnvSlice("KAFKA_TOPICS", []string{"qlik.customers", "qlik.orders"}),
			SecurityProtocol: getEnv("KAFKA_SECURITY_PROTOCOL", "PLAINTEXT"),
			SASLMechanism:    getEnv("KAFKA_SASL_MECHANISM", "PLAIN"),
			SASLUsername:     getEnv("KAFKA_SASL_USERNAME", ""),
			SASLPassword:     getEnv("KAFKA_SASL_PASSWORD", ""),
			SchemaRegistry:   getEnv("SCHEMA_REGISTRY_URL", "http://localhost:8081"),
			AutoOffsetReset:  getEnv("KAFKA_AUTO_OFFSET_RESET", "earliest"),
		},
		MetricsPort: getEnv("METRICS_PORT", defaultMetricsPort),
	}
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvSlice gets environment variable as JSON array with fallback
func getEnvSlice(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		var result []string
		if err := json.Unmarshal([]byte(value), &result); err == nil {
			return result
		}
	}
	return fallback
}
