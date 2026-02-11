#!/bin/bash
# LocalStack initialization script
# This script runs automatically when LocalStack starts

set -e

echo "=== Initializing LocalStack AWS Resources ==="

REGION="us-west-2"
ENDPOINT="http://localhost:4566"

# Wait for services to be ready
echo "Waiting for services..."
sleep 5

# Create DynamoDB tables
echo "Creating DynamoDB tables..."

# Events table
awslocal dynamodb create-table \
    --table-name events \
    --attribute-definitions \
        AttributeName=event_id,AttributeType=S \
        AttributeName=timestamp,AttributeType=N \
    --key-schema \
        AttributeName=event_id,KeyType=HASH \
        AttributeName=timestamp,KeyType=RANGE \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES \
    2>/dev/null || echo "Table 'events' already exists"

# Customers table (for CDC testing)
awslocal dynamodb create-table \
    --table-name customers \
    --attribute-definitions \
        AttributeName=customer_id,AttributeType=S \
    --key-schema \
        AttributeName=customer_id,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES \
    2>/dev/null || echo "Table 'customers' already exists"

# Orders table (for CDC testing)
awslocal dynamodb create-table \
    --table-name orders \
    --attribute-definitions \
        AttributeName=order_id,AttributeType=S \
        AttributeName=customer_id,AttributeType=S \
    --key-schema \
        AttributeName=order_id,KeyType=HASH \
    --global-secondary-indexes \
        "[{\"IndexName\": \"customer-index\", \"KeySchema\": [{\"AttributeName\": \"customer_id\", \"KeyType\": \"HASH\"}], \"Projection\": {\"ProjectionType\": \"ALL\"}, \"ProvisionedThroughput\": {\"ReadCapacityUnits\": 5, \"WriteCapacityUnits\": 5}}]" \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES \
    2>/dev/null || echo "Table 'orders' already exists"

echo "✓ DynamoDB tables created"

# Create SQS queues
echo "Creating SQS queues..."

awslocal sqs create-queue --queue-name event-dlq 2>/dev/null || echo "Queue 'event-dlq' already exists"
awslocal sqs create-queue --queue-name event-processing-queue 2>/dev/null || echo "Queue 'event-processing-queue' already exists"
awslocal sqs create-queue --queue-name cross-region-events 2>/dev/null || echo "Queue 'cross-region-events' already exists"

echo "✓ SQS queues created"

# Create EventBridge event buses
echo "Creating EventBridge event buses..."

awslocal events create-event-bus --name eda-event-bus 2>/dev/null || echo "Event bus 'eda-event-bus' already exists"
awslocal events create-event-bus --name cross-region-bus 2>/dev/null || echo "Event bus 'cross-region-bus' already exists"

# Create EventBridge rules
awslocal events put-rule \
    --name "cdc-events-rule" \
    --event-bus-name "eda-event-bus" \
    --event-pattern '{"source": ["qlik.cdc"]}' \
    2>/dev/null || echo "Rule 'cdc-events-rule' already exists"

awslocal events put-rule \
    --name "transformed-events-rule" \
    --event-bus-name "eda-event-bus" \
    --event-pattern '{"source": ["event-transformer"]}' \
    2>/dev/null || echo "Rule 'transformed-events-rule' already exists"

echo "✓ EventBridge event buses and rules created"

# Create Secrets Manager secrets
echo "Creating Secrets Manager secrets..."

awslocal secretsmanager create-secret \
    --name jwt-secret \
    --secret-string "dev-secret-key-do-not-use-in-production" \
    2>/dev/null || echo "Secret 'jwt-secret' already exists"

awslocal secretsmanager create-secret \
    --name kafka-credentials \
    --secret-string '{"username":"admin","password":"admin-secret","bootstrap_servers":"kafka:29092"}' \
    2>/dev/null || echo "Secret 'kafka-credentials' already exists"

awslocal secretsmanager create-secret \
    --name database-credentials \
    --secret-string '{"host":"localhost","port":5432,"username":"admin","password":"admin-secret","database":"events"}' \
    2>/dev/null || echo "Secret 'database-credentials' already exists"

echo "✓ Secrets Manager secrets created"

# List created resources
echo ""
echo "=== LocalStack Resources Summary ==="
echo "DynamoDB Tables:"
awslocal dynamodb list-tables --output text

echo ""
echo "SQS Queues:"
awslocal sqs list-queues --output text 2>/dev/null || echo "No queues"

echo ""
echo "EventBridge Event Buses:"
awslocal events list-event-buses --output text

echo ""
echo "Secrets:"
awslocal secretsmanager list-secrets --query 'SecretList[].Name' --output text

echo ""
echo "=== LocalStack Initialization Complete ==="
