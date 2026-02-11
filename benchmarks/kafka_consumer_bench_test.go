package benchmarks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wgu/go-performance-enablement/pkg/events"
)

// BenchmarkBaseEventCreation benchmarks creating new base events
func BenchmarkBaseEventCreation(b *testing.B) {
	payload := map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
		"first_name":  "John",
		"last_name":   "Doe",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = events.NewBaseEvent("customer.created", "us-west-2", payload)
	}
}

// BenchmarkBaseEventSerialization benchmarks JSON serialization
func BenchmarkBaseEventSerialization(b *testing.B) {
	event := events.NewBaseEvent("customer.created", "us-west-2", map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
		"first_name":  "John",
		"last_name":   "Doe",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = event.ToJSON()
	}
}

// BenchmarkBaseEventDeserialization benchmarks JSON deserialization
func BenchmarkBaseEventDeserialization(b *testing.B) {
	event := events.NewBaseEvent("customer.created", "us-west-2", map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
	})
	jsonData, _ := event.ToJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = events.FromJSON(jsonData)
	}
}

// BenchmarkCDCEventCreation benchmarks CDC event creation
func BenchmarkCDCEventCreation(b *testing.B) {
	after := map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
		"first_name":  "John",
		"last_name":   "Doe",
		"created_at":  time.Now().Unix(),
	}
	before := map[string]interface{}{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = events.NewCDCEvent(events.OperationInsert, "customers", after, before)
	}
}

// BenchmarkLargePayloadSerialization benchmarks serializing large payloads
func BenchmarkLargePayloadSerialization(b *testing.B) {
	// Create a large payload
	items := make([]map[string]interface{}, 100)
	for i := range items {
		items[i] = map[string]interface{}{
			"item_id":     i,
			"name":        "Product Name That Is Reasonably Long",
			"description": "A detailed description of the product that contains multiple sentences and provides useful information to the customer.",
			"price":       99.99,
			"quantity":    10,
		}
	}

	event := events.NewBaseEvent("order.created", "us-west-2", map[string]interface{}{
		"order_id":    "order-12345",
		"customer_id": "cust-67890",
		"items":       items,
		"total":       9999.00,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = event.ToJSON()
	}
}

// BenchmarkCrossRegionEventCreation benchmarks cross-region event creation
func BenchmarkCrossRegionEventCreation(b *testing.B) {
	base := events.NewBaseEvent("customer.created", "us-west-2", map[string]interface{}{
		"customer_id": "cust-12345",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &events.CrossRegionEvent{
			BaseEvent:         *base,
			TargetRegion:      "us-east-1",
			OriginalTimestamp: time.Now(),
			CompressionType:   "zstd",
		}
	}
}

// BenchmarkJSONMarshalDirect benchmarks direct JSON marshaling vs ToJSON method
func BenchmarkJSONMarshalDirect(b *testing.B) {
	event := events.NewBaseEvent("customer.created", "us-west-2", map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(event)
	}
}

// BenchmarkParallelEventCreation benchmarks concurrent event creation
func BenchmarkParallelEventCreation(b *testing.B) {
	payload := map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = events.NewBaseEvent("customer.created", "us-west-2", payload)
		}
	})
}

// BenchmarkParallelSerialization benchmarks concurrent serialization
func BenchmarkParallelSerialization(b *testing.B) {
	event := events.NewBaseEvent("customer.created", "us-west-2", map[string]interface{}{
		"customer_id": "cust-12345",
		"email":       "test@example.com",
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = event.ToJSON()
		}
	})
}
