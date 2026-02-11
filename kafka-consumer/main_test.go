package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv_WithValue(t *testing.T) {
	key := "TEST_ENV_VAR"
	expected := "test_value"
	fallback := "fallback_value"
	
	os.Setenv(key, expected)
	defer os.Unsetenv(key)
	
	result := getEnv(key, fallback)
	
	assert.Equal(t, expected, result)
}

func TestGetEnv_WithoutValue(t *testing.T) {
	key := "NONEXISTENT_ENV_VAR"
	fallback := "fallback_value"
	
	// Make sure the key doesn't exist
	os.Unsetenv(key)
	
	result := getEnv(key, fallback)
	
	assert.Equal(t, fallback, result)
}

func TestGetEnv_EmptyString(t *testing.T) {
	key := "EMPTY_ENV_VAR"
	fallback := "fallback_value"
	
	os.Setenv(key, "")
	defer os.Unsetenv(key)
	
	result := getEnv(key, fallback)
	
	// Empty string should return fallback
	assert.Equal(t, fallback, result)
}

func TestGetEnvSlice_ValidJSON(t *testing.T) {
	key := "TEST_SLICE_VAR"
	value := `["topic1", "topic2", "topic3"]`
	fallback := []string{"fallback1", "fallback2"}
	
	os.Setenv(key, value)
	defer os.Unsetenv(key)
	
	result := getEnvSlice(key, fallback)
	
	assert.Len(t, result, 3)
	assert.Equal(t, "topic1", result[0])
	assert.Equal(t, "topic2", result[1])
	assert.Equal(t, "topic3", result[2])
}

func TestGetEnvSlice_InvalidJSON(t *testing.T) {
	key := "TEST_SLICE_VAR"
	value := `invalid json`
	fallback := []string{"fallback1", "fallback2"}
	
	os.Setenv(key, value)
	defer os.Unsetenv(key)
	
	result := getEnvSlice(key, fallback)
	
	// Invalid JSON should return fallback
	assert.Equal(t, fallback, result)
}

func TestGetEnvSlice_NotSet(t *testing.T) {
	key := "NONEXISTENT_SLICE_VAR"
	fallback := []string{"fallback1", "fallback2"}
	
	// Make sure the key doesn't exist
	os.Unsetenv(key)
	
	result := getEnvSlice(key, fallback)
	
	assert.Equal(t, fallback, result)
}

func TestGetEnvSlice_EmptyArray(t *testing.T) {
	key := "EMPTY_ARRAY_VAR"
	value := `[]`
	fallback := []string{"fallback1"}
	
	os.Setenv(key, value)
	defer os.Unsetenv(key)
	
	result := getEnvSlice(key, fallback)
	
	assert.Empty(t, result)
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all relevant environment variables
	envVars := []string{
		"KAFKA_BOOTSTRAP_SERVERS",
		"KAFKA_GROUP_ID",
		"KAFKA_TOPICS",
		"KAFKA_SECURITY_PROTOCOL",
		"KAFKA_SASL_MECHANISM",
		"KAFKA_SASL_USERNAME",
		"KAFKA_SASL_PASSWORD",
		"SCHEMA_REGISTRY_URL",
		"KAFKA_AUTO_OFFSET_RESET",
		"METRICS_PORT",
	}
	
	for _, key := range envVars {
		os.Unsetenv(key)
	}
	
	config := loadConfig()
	
	assert.NotNil(t, config)
	assert.NotNil(t, config.KafkaConfig)
	
	// Check default values
	assert.Equal(t, "localhost:9092", config.KafkaConfig.BootstrapServers)
	assert.Equal(t, "go-cdc-consumers", config.KafkaConfig.GroupID)
	assert.Equal(t, []string{"qlik.customers", "qlik.orders"}, config.KafkaConfig.Topics)
	assert.Equal(t, "PLAINTEXT", config.KafkaConfig.SecurityProtocol)
	assert.Equal(t, "PLAIN", config.KafkaConfig.SASLMechanism)
	assert.Equal(t, "", config.KafkaConfig.SASLUsername)
	assert.Equal(t, "", config.KafkaConfig.SASLPassword)
	assert.Equal(t, "http://localhost:8081", config.KafkaConfig.SchemaRegistry)
	assert.Equal(t, "earliest", config.KafkaConfig.AutoOffsetReset)
	assert.Equal(t, defaultMetricsPort, config.MetricsPort)
}

func TestLoadConfig_CustomValues(t *testing.T) {
	// Set custom environment variables
	os.Setenv("KAFKA_BOOTSTRAP_SERVERS", "kafka:9092")
	os.Setenv("KAFKA_GROUP_ID", "custom-group")
	os.Setenv("KAFKA_TOPICS", `["topic1", "topic2"]`)
	os.Setenv("KAFKA_SECURITY_PROTOCOL", "SASL_SSL")
	os.Setenv("KAFKA_SASL_MECHANISM", "SCRAM-SHA-256")
	os.Setenv("KAFKA_SASL_USERNAME", "user")
	os.Setenv("KAFKA_SASL_PASSWORD", "pass")
	os.Setenv("SCHEMA_REGISTRY_URL", "http://registry:8081")
	os.Setenv("KAFKA_AUTO_OFFSET_RESET", "latest")
	os.Setenv("METRICS_PORT", ":8080")
	
	defer func() {
		os.Unsetenv("KAFKA_BOOTSTRAP_SERVERS")
		os.Unsetenv("KAFKA_GROUP_ID")
		os.Unsetenv("KAFKA_TOPICS")
		os.Unsetenv("KAFKA_SECURITY_PROTOCOL")
		os.Unsetenv("KAFKA_SASL_MECHANISM")
		os.Unsetenv("KAFKA_SASL_USERNAME")
		os.Unsetenv("KAFKA_SASL_PASSWORD")
		os.Unsetenv("SCHEMA_REGISTRY_URL")
		os.Unsetenv("KAFKA_AUTO_OFFSET_RESET")
		os.Unsetenv("METRICS_PORT")
	}()
	
	config := loadConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, "kafka:9092", config.KafkaConfig.BootstrapServers)
	assert.Equal(t, "custom-group", config.KafkaConfig.GroupID)
	assert.Equal(t, []string{"topic1", "topic2"}, config.KafkaConfig.Topics)
	assert.Equal(t, "SASL_SSL", config.KafkaConfig.SecurityProtocol)
	assert.Equal(t, "SCRAM-SHA-256", config.KafkaConfig.SASLMechanism)
	assert.Equal(t, "user", config.KafkaConfig.SASLUsername)
	assert.Equal(t, "pass", config.KafkaConfig.SASLPassword)
	assert.Equal(t, "http://registry:8081", config.KafkaConfig.SchemaRegistry)
	assert.Equal(t, "latest", config.KafkaConfig.AutoOffsetReset)
	assert.Equal(t, ":8080", config.MetricsPort)
}

func TestGetEnv_MultipleKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback string
		expected string
	}{
		{
			name:     "with value",
			key:      "KEY1",
			value:    "value1",
			fallback: "fallback1",
			expected: "value1",
		},
		{
			name:     "without value",
			key:      "KEY2",
			value:    "",
			fallback: "fallback2",
			expected: "fallback2",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}
			
			result := getEnv(tt.key, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvSlice_MultipleFormats(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback []string
		expected []string
	}{
		{
			name:     "single element",
			value:    `["single"]`,
			fallback: []string{"fallback"},
			expected: []string{"single"},
		},
		{
			name:     "multiple elements",
			value:    `["one", "two", "three"]`,
			fallback: []string{"fallback"},
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "empty array",
			value:    `[]`,
			fallback: []string{"fallback"},
			expected: []string{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_ARRAY"
			os.Setenv(key, tt.value)
			defer os.Unsetenv(key)
			
			result := getEnvSlice(key, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}
