module github.com/wgu/go-performance-enablement

go 1.21

require (
	// AWS SDK
	github.com/aws/aws-lambda-go v1.46.0
	github.com/aws/aws-sdk-go-v2 v1.24.1
	github.com/aws/aws-sdk-go-v2/config v1.26.6
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.26.9
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.28.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.29.7
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.26.3
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.12.17
	
	// Kafka
	github.com/confluentinc/confluent-kafka-go/v2 v2.3.0
	github.com/linkedin/goavro/v2 v2.12.0
	github.com/riferrei/srclient v0.6.0
	
	// HTTP/API
	github.com/gin-gonic/gin v1.9.1
	github.com/gorilla/mux v1.8.1
	
	// Metrics & Monitoring
	github.com/prometheus/client_golang v1.18.0
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/trace v1.21.0
	go.opentelemetry.io/otel/exporters/prometheus v0.44.0
	
	// Logging
	go.uber.org/zap v1.26.0
	github.com/sirupsen/logrus v1.9.3
	
	// Utilities
	github.com/google/uuid v1.5.0
	github.com/klauspost/compress v1.17.4
	github.com/stretchr/testify v1.8.4
	golang.org/x/sync v0.6.0
)

require (
	github.com/aws/smithy-go v1.19.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
)
