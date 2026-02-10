#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting local development environment setup...${NC}"

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed. Please install Docker first.${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Docker Compose is not installed. Please install Docker Compose first.${NC}"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo -e "${RED}Go is not installed. Please install Go 1.24.3+ first.${NC}"
    exit 1
fi

echo -e "${GREEN}✓ All prerequisites satisfied${NC}"

# Create necessary directories
echo -e "\n${YELLOW}Creating directories...${NC}"
mkdir -p monitoring/grafana/datasources
mkdir -p monitoring/grafana/dashboards
mkdir -p localstack
mkdir -p logs
mkdir -p tests/fixtures

echo -e "${GREEN}✓ Directories created${NC}"

# Start infrastructure services
echo -e "\n${YELLOW}Starting infrastructure services...${NC}"
docker-compose up -d zookeeper kafka schema-registry localstack prometheus grafana

echo -e "${GREEN}✓ Infrastructure services started${NC}"

# Wait for Kafka to be ready
echo -e "\n${YELLOW}Waiting for Kafka to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0

while ! docker-compose exec -T kafka kafka-broker-api-versions --bootstrap-server localhost:9092 &> /dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo -e "${RED}Kafka failed to start after $MAX_RETRIES attempts${NC}"
        exit 1
    fi
    echo "Waiting for Kafka... (attempt $RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

echo -e "${GREEN}✓ Kafka is ready${NC}"

# Create Kafka topics
echo -e "\n${YELLOW}Creating Kafka topics...${NC}"

TOPICS=(
    "qlik.customers:3:1"
    "qlik.orders:3:1"
    "qlik.products:3:1"
    "events.transformed:3:1"
    "events.validation_failed:3:1"
)

for TOPIC_CONFIG in "${TOPICS[@]}"; do
    IFS=':' read -r TOPIC PARTITIONS REPLICATION <<< "$TOPIC_CONFIG"
    
    echo "Creating topic: $TOPIC (partitions: $PARTITIONS, replication: $REPLICATION)"
    
    docker-compose exec -T kafka kafka-topics \
        --create \
        --if-not-exists \
        --topic "$TOPIC" \
        --partitions "$PARTITIONS" \
        --replication-factor "$REPLICATION" \
        --bootstrap-server localhost:9092 || true
done

echo -e "${GREEN}✓ Kafka topics created${NC}"

# Wait for Schema Registry
echo -e "\n${YELLOW}Waiting for Schema Registry...${NC}"
RETRY_COUNT=0

while ! curl -s http://localhost:8081/subjects &> /dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo -e "${RED}Schema Registry failed to start${NC}"
        exit 1
    fi
    echo "Waiting for Schema Registry... (attempt $RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

echo -e "${GREEN}✓ Schema Registry is ready${NC}"

# Register Avro schemas
echo -e "\n${YELLOW}Registering Avro schemas...${NC}"

cat > /tmp/customer-schema.json <<'EOF'
{
  "type": "record",
  "name": "Customer",
  "namespace": "com.wgu.qlik",
  "fields": [
    {"name": "customer_id", "type": "string"},
    {"name": "email", "type": "string"},
    {"name": "first_name", "type": "string"},
    {"name": "last_name", "type": "string"},
    {"name": "created_at", "type": "long"},
    {"name": "updated_at", "type": "long"}
  ]
}
EOF

curl -X POST http://localhost:8081/subjects/qlik.customers-value/versions \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d "{\"schema\":$(jq -c . < /tmp/customer-schema.json | jq -R .)}" \
    &> /dev/null || true

echo -e "${GREEN}✓ Schemas registered${NC}"

# Wait for LocalStack
echo -e "\n${YELLOW}Waiting for LocalStack...${NC}"
RETRY_COUNT=0

while ! curl -s http://localhost:4566/_localstack/health | grep -q "available"; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo -e "${RED}LocalStack failed to start${NC}"
        exit 1
    fi
    echo "Waiting for LocalStack... (attempt $RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

echo -e "${GREEN}✓ LocalStack is ready${NC}"

# Create AWS resources in LocalStack
echo -e "\n${YELLOW}Creating AWS resources in LocalStack...${NC}"

AWS_ENDPOINT="http://localhost:4566"
AWS_REGION="us-west-2"

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=$AWS_REGION

# Create DynamoDB table
echo "Creating DynamoDB table: events"
aws dynamodb create-table \
    --endpoint-url $AWS_ENDPOINT \
    --table-name events \
    --attribute-definitions \
        AttributeName=event_id,AttributeType=S \
        AttributeName=timestamp,AttributeType=N \
    --key-schema \
        AttributeName=event_id,KeyType=HASH \
        AttributeName=timestamp,KeyType=RANGE \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    &> /dev/null || true

# Create SQS queues
echo "Creating SQS DLQ: event-dlq"
aws sqs create-queue \
    --endpoint-url $AWS_ENDPOINT \
    --queue-name event-dlq \
    &> /dev/null || true

# Create EventBridge event bus
echo "Creating EventBridge bus: eda-event-bus"
aws events create-event-bus \
    --endpoint-url $AWS_ENDPOINT \
    --name eda-event-bus \
    &> /dev/null || true

# Create Secrets Manager secret
echo "Creating secret: jwt-secret"
aws secretsmanager create-secret \
    --endpoint-url $AWS_ENDPOINT \
    --name jwt-secret \
    --secret-string "dev-secret-key-do-not-use-in-production" \
    &> /dev/null || true

echo -e "${GREEN}✓ AWS resources created${NC}"

# Build Go application
echo -e "\n${YELLOW}Building Go applications...${NC}"
go mod download
go build -o bin/kafka-consumer ./kafka-consumer/main.go

echo -e "${GREEN}✓ Go applications built${NC}"

# Start application services
echo -e "\n${YELLOW}Starting application services...${NC}"
docker-compose up -d kafka-consumer qlik-simulator kafka-ui

echo -e "${GREEN}✓ Application services started${NC}"

# Print access URLs
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Local Development Environment Ready!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "Service URLs:"
echo -e "  ${YELLOW}Kafka UI:${NC}          http://localhost:8080"
echo -e "  ${YELLOW}Grafana:${NC}           http://localhost:3000 (admin/admin)"
echo -e "  ${YELLOW}Prometheus:${NC}        http://localhost:9090"
echo -e "  ${YELLOW}Schema Registry:${NC}   http://localhost:8081"
echo -e "  ${YELLOW}LocalStack:${NC}        http://localhost:4566"
echo -e "  ${YELLOW}Kafka Consumer:${NC}    http://localhost:9091/metrics"
echo ""
echo -e "Kafka:"
echo -e "  ${YELLOW}Bootstrap:${NC}         localhost:9092"
echo -e "  ${YELLOW}Topics:${NC}            qlik.customers, qlik.orders, qlik.products"
echo ""
echo -e "Next steps:"
echo -e "  1. View logs:           ${YELLOW}docker-compose logs -f kafka-consumer${NC}"
echo -e "  2. Run tests:           ${YELLOW}go test ./...${NC}"
echo -e "  3. Stop services:       ${YELLOW}docker-compose down${NC}"
echo ""
echo -e "${GREEN}Happy coding!${NC}"
