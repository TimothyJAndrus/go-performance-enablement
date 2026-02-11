#!/bin/bash
# Kafka initialization script
# Creates topics and registers Avro schemas with Schema Registry

set -e

echo "=== Initializing Kafka ==="

KAFKA_BOOTSTRAP="kafka:29092"
SCHEMA_REGISTRY_URL="http://schema-registry:8081"

# Wait for Kafka to be ready
echo "Waiting for Kafka to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0

while ! kafka-broker-api-versions --bootstrap-server $KAFKA_BOOTSTRAP &>/dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "ERROR: Kafka failed to start after $MAX_RETRIES attempts"
        exit 1
    fi
    echo "Waiting for Kafka... (attempt $RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

echo "✓ Kafka is ready"

# Create topics
echo "Creating Kafka topics..."

TOPICS=(
    "qlik.customers:3:1"
    "qlik.orders:3:1"
    "qlik.products:3:1"
    "events.transformed:3:1"
    "events.validation_failed:3:1"
    "events.dead_letter:3:1"
    "health.checks:1:1"
)

for TOPIC_CONFIG in "${TOPICS[@]}"; do
    IFS=':' read -r TOPIC PARTITIONS REPLICATION <<< "$TOPIC_CONFIG"
    
    echo "Creating topic: $TOPIC (partitions: $PARTITIONS, replication: $REPLICATION)"
    
    kafka-topics \
        --create \
        --if-not-exists \
        --topic "$TOPIC" \
        --partitions "$PARTITIONS" \
        --replication-factor "$REPLICATION" \
        --bootstrap-server $KAFKA_BOOTSTRAP \
        2>/dev/null || echo "Topic '$TOPIC' already exists"
done

echo "✓ Kafka topics created"

# Wait for Schema Registry
echo "Waiting for Schema Registry..."
RETRY_COUNT=0

while ! curl -s "$SCHEMA_REGISTRY_URL/subjects" &>/dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "ERROR: Schema Registry failed to start after $MAX_RETRIES attempts"
        exit 1
    fi
    echo "Waiting for Schema Registry... (attempt $RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

echo "✓ Schema Registry is ready"

# Register Avro schemas
echo "Registering Avro schemas..."

# Customer CDC schema
CUSTOMER_SCHEMA='{
  "type": "record",
  "name": "CustomerCDC",
  "namespace": "com.wgu.qlik",
  "fields": [
    {"name": "operation", "type": "string", "doc": "CDC operation: INSERT, UPDATE, DELETE, REFRESH"},
    {"name": "table_name", "type": "string"},
    {"name": "timestamp", "type": "long", "logicalType": "timestamp-millis"},
    {"name": "before", "type": ["null", "string"], "default": null, "doc": "Previous record state (JSON)"},
    {"name": "after", "type": ["null", "string"], "default": null, "doc": "New record state (JSON)"},
    {"name": "customer_id", "type": "string"},
    {"name": "email", "type": ["null", "string"], "default": null},
    {"name": "first_name", "type": ["null", "string"], "default": null},
    {"name": "last_name", "type": ["null", "string"], "default": null},
    {"name": "created_at", "type": ["null", "long"], "default": null},
    {"name": "updated_at", "type": ["null", "long"], "default": null}
  ]
}'

curl -X POST "$SCHEMA_REGISTRY_URL/subjects/qlik.customers-value/versions" \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d "{\"schema\": $(echo "$CUSTOMER_SCHEMA" | jq -c . | jq -R .)}" \
    2>/dev/null && echo "✓ Registered schema: qlik.customers-value" || echo "Schema qlik.customers-value already exists"

# Order CDC schema
ORDER_SCHEMA='{
  "type": "record",
  "name": "OrderCDC",
  "namespace": "com.wgu.qlik",
  "fields": [
    {"name": "operation", "type": "string"},
    {"name": "table_name", "type": "string"},
    {"name": "timestamp", "type": "long", "logicalType": "timestamp-millis"},
    {"name": "before", "type": ["null", "string"], "default": null},
    {"name": "after", "type": ["null", "string"], "default": null},
    {"name": "order_id", "type": "string"},
    {"name": "customer_id", "type": "string"},
    {"name": "total_amount", "type": ["null", "double"], "default": null},
    {"name": "status", "type": ["null", "string"], "default": null},
    {"name": "created_at", "type": ["null", "long"], "default": null},
    {"name": "updated_at", "type": ["null", "long"], "default": null}
  ]
}'

curl -X POST "$SCHEMA_REGISTRY_URL/subjects/qlik.orders-value/versions" \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d "{\"schema\": $(echo "$ORDER_SCHEMA" | jq -c . | jq -R .)}" \
    2>/dev/null && echo "✓ Registered schema: qlik.orders-value" || echo "Schema qlik.orders-value already exists"

# Product CDC schema
PRODUCT_SCHEMA='{
  "type": "record",
  "name": "ProductCDC",
  "namespace": "com.wgu.qlik",
  "fields": [
    {"name": "operation", "type": "string"},
    {"name": "table_name", "type": "string"},
    {"name": "timestamp", "type": "long", "logicalType": "timestamp-millis"},
    {"name": "before", "type": ["null", "string"], "default": null},
    {"name": "after", "type": ["null", "string"], "default": null},
    {"name": "product_id", "type": "string"},
    {"name": "name", "type": ["null", "string"], "default": null},
    {"name": "description", "type": ["null", "string"], "default": null},
    {"name": "price", "type": ["null", "double"], "default": null},
    {"name": "category", "type": ["null", "string"], "default": null},
    {"name": "created_at", "type": ["null", "long"], "default": null},
    {"name": "updated_at", "type": ["null", "long"], "default": null}
  ]
}'

curl -X POST "$SCHEMA_REGISTRY_URL/subjects/qlik.products-value/versions" \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d "{\"schema\": $(echo "$PRODUCT_SCHEMA" | jq -c . | jq -R .)}" \
    2>/dev/null && echo "✓ Registered schema: qlik.products-value" || echo "Schema qlik.products-value already exists"

# Transformed event schema
TRANSFORMED_SCHEMA='{
  "type": "record",
  "name": "TransformedEvent",
  "namespace": "com.wgu.events",
  "fields": [
    {"name": "event_id", "type": "string"},
    {"name": "event_type", "type": "string"},
    {"name": "source_region", "type": "string"},
    {"name": "timestamp", "type": "long", "logicalType": "timestamp-millis"},
    {"name": "correlation_id", "type": ["null", "string"], "default": null},
    {"name": "trace_id", "type": ["null", "string"], "default": null},
    {"name": "payload", "type": "string", "doc": "JSON payload"}
  ]
}'

curl -X POST "$SCHEMA_REGISTRY_URL/subjects/events.transformed-value/versions" \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d "{\"schema\": $(echo "$TRANSFORMED_SCHEMA" | jq -c . | jq -R .)}" \
    2>/dev/null && echo "✓ Registered schema: events.transformed-value" || echo "Schema events.transformed-value already exists"

echo ""
echo "=== Kafka Topics Summary ==="
kafka-topics --list --bootstrap-server $KAFKA_BOOTSTRAP

echo ""
echo "=== Schema Registry Subjects ==="
curl -s "$SCHEMA_REGISTRY_URL/subjects" | jq .

echo ""
echo "=== Kafka Initialization Complete ==="
