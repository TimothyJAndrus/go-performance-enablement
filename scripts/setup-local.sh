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
mkdir -p logs
mkdir -p tests/fixtures
mkdir -p bin

echo -e "${GREEN}✓ Directories created${NC}"

# Start infrastructure services
echo -e "\n${YELLOW}Starting infrastructure services...${NC}"
docker-compose up -d zookeeper kafka schema-registry localstack prometheus grafana

echo -e "${GREEN}✓ Infrastructure services started${NC}"

# Run Kafka initialization (creates topics and registers schemas)
echo -e "\n${YELLOW}Running Kafka initialization...${NC}"
docker-compose up kafka-init
echo -e "${GREEN}✓ Kafka initialization complete (topics and schemas created)${NC}"

# Note: Kafka topics and schemas are now created by kafka-init container
# See docker/kafka-init/init-kafka.sh for details

# Wait for LocalStack (resources are created automatically by init script)
echo -e "\n${YELLOW}Waiting for LocalStack...${NC}"
MAX_RETRIES=30
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
echo -e "${GREEN}  (AWS resources created automatically by docker/localstack-init/init-aws.sh)${NC}"

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
