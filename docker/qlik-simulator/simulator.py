#!/usr/bin/env python3
"""
Qlik CDC Simulator - Generates test CDC events for Kafka topics.
Simulates Qlik Replicate CDC message format.
"""

import json
import os
import random
import time
import uuid
from datetime import datetime
from typing import Any

from confluent_kafka import Producer
from faker import Faker

fake = Faker()


def get_env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


KAFKA_BOOTSTRAP_SERVERS = get_env("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
SCHEMA_REGISTRY_URL = get_env("SCHEMA_REGISTRY_URL", "http://localhost:8081")
TOPICS = get_env("TOPICS", "qlik.customers,qlik.orders").split(",")
EVENT_RATE = int(get_env("EVENT_RATE", "10"))  # events per second


def create_producer() -> Producer:
    """Create Kafka producer with retry configuration."""
    config = {
        "bootstrap.servers": KAFKA_BOOTSTRAP_SERVERS,
        "client.id": "qlik-simulator",
        "acks": "all",
        "retries": 5,
        "retry.backoff.ms": 1000,
    }
    return Producer(config)


def delivery_report(err, msg):
    """Callback for message delivery reports."""
    if err is not None:
        print(f"Message delivery failed: {err}")
    else:
        print(f"Message delivered to {msg.topic()} [{msg.partition()}] @ {msg.offset()}")


def generate_customer_event() -> dict[str, Any]:
    """Generate a CDC event for the customers table."""
    operation = random.choices(["INSERT", "UPDATE", "DELETE"], weights=[0.5, 0.4, 0.1])[0]
    customer_id = str(uuid.uuid4())
    
    before_image = None
    after_image = None
    
    customer_data = {
        "customer_id": customer_id,
        "first_name": fake.first_name(),
        "last_name": fake.last_name(),
        "email": fake.email(),
        "phone": fake.phone_number(),
        "address": fake.street_address(),
        "city": fake.city(),
        "state": fake.state_abbr(),
        "zip_code": fake.zipcode(),
        "created_at": fake.date_time_this_year().isoformat(),
        "updated_at": datetime.utcnow().isoformat(),
    }
    
    if operation == "INSERT":
        after_image = customer_data
    elif operation == "UPDATE":
        before_image = {**customer_data, "email": fake.email()}
        after_image = customer_data
    else:  # DELETE
        before_image = customer_data
    
    return {
        "header": {
            "operation": operation,
            "timestamp": datetime.utcnow().isoformat(),
            "transaction_id": str(uuid.uuid4()),
            "schema": "public",
            "table": "customers",
        },
        "before": before_image,
        "after": after_image,
    }


def generate_order_event() -> dict[str, Any]:
    """Generate a CDC event for the orders table."""
    operation = random.choices(["INSERT", "UPDATE", "DELETE"], weights=[0.6, 0.35, 0.05])[0]
    order_id = str(uuid.uuid4())
    customer_id = str(uuid.uuid4())
    
    before_image = None
    after_image = None
    
    order_data = {
        "order_id": order_id,
        "customer_id": customer_id,
        "order_date": fake.date_time_this_month().isoformat(),
        "status": random.choice(["pending", "processing", "shipped", "delivered", "cancelled"]),
        "total_amount": round(random.uniform(10.0, 1000.0), 2),
        "currency": "USD",
        "shipping_address": fake.street_address(),
        "shipping_city": fake.city(),
        "shipping_state": fake.state_abbr(),
        "shipping_zip": fake.zipcode(),
        "items_count": random.randint(1, 10),
        "created_at": fake.date_time_this_month().isoformat(),
        "updated_at": datetime.utcnow().isoformat(),
    }
    
    if operation == "INSERT":
        after_image = order_data
    elif operation == "UPDATE":
        before_image = {**order_data, "status": "pending"}
        after_image = order_data
    else:  # DELETE
        before_image = order_data
    
    return {
        "header": {
            "operation": operation,
            "timestamp": datetime.utcnow().isoformat(),
            "transaction_id": str(uuid.uuid4()),
            "schema": "public",
            "table": "orders",
        },
        "before": before_image,
        "after": after_image,
    }


TOPIC_GENERATORS = {
    "qlik.customers": generate_customer_event,
    "qlik.orders": generate_order_event,
}


def wait_for_kafka(producer: Producer, max_retries: int = 30, retry_interval: int = 2):
    """Wait for Kafka to be ready."""
    print(f"Waiting for Kafka at {KAFKA_BOOTSTRAP_SERVERS}...")
    for i in range(max_retries):
        try:
            metadata = producer.list_topics(timeout=5)
            print(f"Connected to Kafka. Available topics: {list(metadata.topics.keys())}")
            return True
        except Exception as e:
            print(f"Attempt {i + 1}/{max_retries}: Kafka not ready - {e}")
            time.sleep(retry_interval)
    raise Exception("Failed to connect to Kafka after maximum retries")


def main():
    print("=" * 60)
    print("Qlik CDC Simulator Starting")
    print("=" * 60)
    print(f"Kafka Bootstrap Servers: {KAFKA_BOOTSTRAP_SERVERS}")
    print(f"Schema Registry URL: {SCHEMA_REGISTRY_URL}")
    print(f"Topics: {TOPICS}")
    print(f"Event Rate: {EVENT_RATE} events/second")
    print("=" * 60)
    
    producer = create_producer()
    wait_for_kafka(producer)
    
    interval = 1.0 / EVENT_RATE if EVENT_RATE > 0 else 1.0
    event_count = 0
    
    print(f"\nStarting event generation (interval: {interval:.3f}s)...")
    
    try:
        while True:
            topic = random.choice(TOPICS)
            generator = TOPIC_GENERATORS.get(topic.strip())
            
            if generator:
                event = generator()
                key = event["header"]["transaction_id"]
                value = json.dumps(event)
                
                producer.produce(
                    topic=topic.strip(),
                    key=key.encode("utf-8"),
                    value=value.encode("utf-8"),
                    callback=delivery_report,
                )
                producer.poll(0)
                
                event_count += 1
                if event_count % 100 == 0:
                    print(f"Generated {event_count} events...")
            
            time.sleep(interval)
            
    except KeyboardInterrupt:
        print(f"\nShutting down. Total events generated: {event_count}")
    finally:
        producer.flush(timeout=10)


if __name__ == "__main__":
    main()
