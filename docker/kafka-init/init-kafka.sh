#!/bin/bash
# Kafka initialization script
# Creates topics and registers Avro schemas with Schema Registry

set -e

echo "=== Initializing Kafka ==="

KAFKA_BOOTSTRAP="kafka:29092"
SCHEMA_REGISTRY_URL="http://schema-registry:8081"

# Wait for Kafka to be ready
echo "Waiting for Kafka to be ready..."
MAX_KAFKA_RETRIES=60
MAX_SR_RETRIES=90
RETRY_INTERVAL=3
RETRY_COUNT=0

while ! kafka-broker-api-versions --bootstrap-server $KAFKA_BOOTSTRAP &>/dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_KAFKA_RETRIES ]; then
        echo "ERROR: Kafka failed to start after $MAX_KAFKA_RETRIES attempts"
        exit 1
    fi
    echo "Waiting for Kafka... (attempt $RETRY_COUNT/$MAX_KAFKA_RETRIES)"
    sleep $RETRY_INTERVAL
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
echo "(Schema Registry can take a while to initialize its internal topics)"
RETRY_COUNT=0

while true; do
    # Check if Schema Registry is responding with a valid JSON response
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$SCHEMA_REGISTRY_URL/subjects" 2>/dev/null || echo "000")
    
    if [ "$RESPONSE" = "200" ]; then
        break
    fi
    
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_SR_RETRIES ]; then
        echo "ERROR: Schema Registry failed to start after $MAX_SR_RETRIES attempts (HTTP: $RESPONSE)"
        echo "Debugging info:"
        curl -v "$SCHEMA_REGISTRY_URL/subjects" 2>&1 || true
        exit 1
    fi
    echo "Waiting for Schema Registry... (attempt $RETRY_COUNT/$MAX_SR_RETRIES, HTTP: $RESPONSE)"
    sleep $RETRY_INTERVAL
done

echo "✓ Schema Registry is ready"

# Register Avro schemas
echo "Registering Avro schemas..."

# Helper function to escape JSON for Schema Registry (no jq dependency)
# Compacts JSON and escapes quotes for embedding in the "schema" field
escape_schema() {
    # Remove newlines and extra spaces, then escape double quotes
    echo "$1" | tr -d '\n' | sed 's/  */ /g' | sed 's/"/\\"/g'
}

# Function to register a schema
register_schema() {
    local subject="$1"
    local schema="$2"
    local escaped_schema
    escaped_schema=$(escape_schema "$schema")
    
    local response
    response=$(curl -s -w "\n%{http_code}" -X POST "$SCHEMA_REGISTRY_URL/subjects/${subject}/versions" \
        -H "Content-Type: application/vnd.schemaregistry.v1+json" \
        -d "{\"schema\": \"${escaped_schema}\"}" 2>/dev/null)
    
    local http_code
    http_code=$(echo "$response" | tail -n1)
    local body
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "409" ]; then
        echo "✓ Registered schema: $subject"
    else
        echo "⚠ Schema $subject registration returned HTTP $http_code: $body"
    fi
}

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
register_schema "qlik.customers-value" "$CUSTOMER_SCHEMA"

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
register_schema "qlik.orders-value" "$ORDER_SCHEMA"

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
register_schema "qlik.products-value" "$PRODUCT_SCHEMA"

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
register_schema "events.transformed-value" "$TRANSFORMED_SCHEMA"

echo ""
echo "=== Kafka Topics Summary ==="
kafka-topics --list --bootstrap-server $KAFKA_BOOTSTRAP

echo ""
echo "=== Schema Registry Subjects ==="
curl -s "$SCHEMA_REGISTRY_URL/subjects"

echo ""
echo "=== Kafka Initialization Complete ==="
