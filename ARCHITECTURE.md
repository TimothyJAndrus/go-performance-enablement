# Architecture - Go Performance Enablement

## Overview

This document describes the architecture of the Go-based multi-region event-driven system for WGU. The system provides high-performance, reliable event processing across us-west-2 and us-east-1 regions with automatic disaster recovery.

## System Architecture

### High-Level Architecture

``` TXT
┌─────────────────────────────────────────────────────────────────┐
│                        Multi-Region EDA                          │
│                                                                   │
│  ┌────────────────────┐         ┌────────────────────┐          │
│  │   us-west-2        │         │   us-east-1        │          │
│  │   (Primary)        │◄────────┤   (Secondary)      │          │
│  │                    │         │                    │          │
│  │  ┌──────────────┐  │         │  ┌──────────────┐  │          │
│  │  │ Lambda       │  │         │  │ Lambda       │  │          │
│  │  │ Functions    │  │         │  │ Functions    │  │          │
│  │  └──────────────┘  │         │  └──────────────┘  │          │
│  │                    │         │                    │          │
│  │  ┌──────────────┐  │         │  ┌──────────────┐  │          │
│  │  │ EKS Cluster  │  │         │  │ EKS Cluster  │  │          │
│  │  │ (Kafka)      │  │         │  │ (Kafka)      │  │          │
│  │  └──────────────┘  │         │  └──────────────┘  │          │
│  │                    │         │                    │          │
│  │  ┌──────────────┐  │         │  ┌──────────────┐  │          │
│  │  │ DynamoDB     │◄─┼─────────┼─►│ DynamoDB     │  │          │
│  │  │ (Global)     │  │         │  │ (Global)     │  │          │
│  │  └──────────────┘  │         │  └──────────────┘  │          │
│  └────────────────────┘         └────────────────────┘          │
│                                                                   │
│                    ┌──────────────────┐                          │
│                    │ Confluent Cloud  │                          │
│                    │ Kafka Cluster    │                          │
│                    │ (Cluster Linking)│                          │
│                    └──────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### 1. Lambda Functions (Go)

#### Event Router

**Purpose**: Routes events between regions with circuit breaker pattern  
**Runtime**: Go 1.24.3 on provided.al2023  
**Memory**: 128MB  
**Timeout**: 30s

**Key Features**:

- Circuit breaker (Closed → Open → Half-Open)
- Zstd compression for cross-region transfer
- Dead letter queue integration
- Prometheus metrics export

**Performance**:

- Cold start: 100-150ms
- Warm execution: 8-12ms (p99)
- Throughput: 8,000-10,000 req/s

**Code Structure**:

```go
lambdas/event-router/
├── main.go              # Handler and circuit breaker
└── go.mod              # Dependencies
```

#### Stream Processor

**Purpose**: Processes DynamoDB Streams for CDC replication  
**Runtime**: Go 1.24.3 on provided.al2023  
**Memory**: 256MB  
**Timeout**: 60s

**Key Features**:

- Handles INSERT, UPDATE, DELETE operations
- Cross-region replication
- EventBridge event publishing
- DLQ for failed events

**Performance**:

- Batch size: 100 records
- Processing latency: 10-15ms per record
- Error rate: <0.1%

#### Event Transformer

**Purpose**: Validates, enriches, and normalizes events  
**Runtime**: Go 1.24.3 on provided.al2023  
**Memory**: 512MB  
**Timeout**: 60s

**Key Features**:

- Regex-based validation (email, UUID, etc.)
- Field enrichment with geolocation
- Data normalization (lowercase, formatting)
- Separate streams for valid/invalid events

**Validation Rules**:

- Required fields: event_id, event_type, source_region, timestamp
- Email format validation
- Timestamp bounds checking
- Custom business rules

#### Health Checker

**Purpose**: Aggregates health status across regions  
**Runtime**: Go 1.24.3 on provided.al2023  
**Memory**: 256MB  
**Timeout**: 30s  
**Schedule**: Every 5 minutes (EventBridge)

**Health Checks**:

- DynamoDB: ListTables API call
- EventBridge: ListEventBuses API call
- SQS: ListQueues API call
- Latency threshold: 500ms (degraded), timeout (unhealthy)

**Output**: Aggregated HealthCheckEvent published to EventBridge

#### API Authorizer

**Purpose**: JWT validation for API Gateway  
**Runtime**: Go 1.24.3 on provided.al2023  
**Memory**: 128MB  
**Timeout**: 5s

**Key Features**:

- HMAC-SHA256 and RSA-256 support
- Issuer and audience validation
- Token expiration checking
- Context propagation (userId, roles, tenantId)

**Performance Target**: <10ms authorization latency

### 2. Kafka Consumer (EKS)

**Purpose**: High-throughput CDC event processing from Qlik  
**Runtime**: Go 1.24.3 in Docker container  
**Deployment**: Kubernetes Deployment with HPA

**Architecture**:

``` TXT
kafka-consumer/
├── main.go                    # Entry point, config
├── consumer/
│   └── kafka.go              # Confluent Kafka consumer
├── processor/
│   └── cdc.go                # CDC event processing
└── Dockerfile                # Multi-stage build
```

**Key Features**:

- Manual offset commits for exactly-once semantics
- Graceful shutdown with signal handling
- Prometheus metrics on :9090/metrics
- Health and readiness probes
- Avro deserialization with Schema Registry

**Performance**:

- Throughput: 8,000-12,000 msgs/sec per pod
- Latency: <15ms p99 processing time
- Memory: 200-400MB per pod
- Consumer lag: <1,000 messages sustained

**Kubernetes Configuration**:

- Min replicas: 3
- Max replicas: 10
- HPA: CPU 70%, Memory 80%, Consumer lag
- PodDisruptionBudget: minAvailable=1
- Resource requests: 500m CPU, 512Mi memory
- Resource limits: 2000m CPU, 2Gi memory

### 3. Shared Packages

#### pkg/events

**Purpose**: Common event types and schemas

**Types**:

- `BaseEvent`: Core event structure
- `CrossRegionEvent`: Cross-region wrapper with compression
- `CDCEvent`: Change Data Capture from Qlik
- `EventRecord`: DynamoDB Streams record
- `HealthCheckEvent`: Multi-region health status
- `TransformedEvent`: Validated/enriched event
- `DeadLetterEvent`: Failed event wrapper

#### pkg/awsutils

**Purpose**: AWS SDK helpers and utilities

**Components**:

- `AWSClients`: Client initialization for all services
- `EventBridgePublisher`: Event publishing with retry
- `DynamoDBHelper`: CRUD operations wrapper
- Secret retrieval from Secrets Manager
- DLQ message handling

#### pkg/metrics

**Purpose**: Prometheus metrics collection

**Metrics**:

- Lambda: invocations, errors, duration
- Kafka: messages consumed, lag, processing duration
- CDC: events processed by operation type
- EventBridge: events published, errors
- Circuit breaker: state, failures
- DynamoDB: operations, errors
- Cross-region: event count, latency
- DLQ: message count by source

## Data Flow

### 1. CDC Event Processing Flow

``` TXT
Qlik → Kafka Topic → Schema Registry → Kafka Consumer (EKS)
                                             ↓
                                    Process CDC Event
                                             ↓
                          ┌──────────────────┴──────────────────┐
                          ↓                                     ↓
                    DynamoDB Update                    EventBridge Publish
                          ↓                                     ↓
                  DynamoDB Streams               Downstream Consumers
                          ↓
                Stream Processor Lambda
                          ↓
              ┌───────────┴────────────┐
              ↓                        ↓
      EventBridge Event      Cross-Region Replication
```

### 2. Cross-Region Event Flow

``` TXT
Region A Event → DynamoDB → DynamoDB Streams → Event Router Lambda
                                                         ↓
                                                Circuit Breaker Check
                                                         ↓
                                                  Compress (zstd)
                                                         ↓
                                          EventBridge (Region B)
                                                         ↓
                                              Region B Consumers
```

### 3. Event Transformation Flow

``` TXT
Raw Event → EventBridge → Event Transformer Lambda
                                    ↓
                         ┌──────────┴──────────┐
                         ↓                     ↓
                    Validate              Enrich
                         ↓                     ↓
                  ┌──────┴──────┐         Geolocation
                  ↓             ↓         Metadata
            Valid?          Invalid       ↓
              ↓                ↓          Normalize
        Normalize         Error Stream    ↓
              ↓                           ↓
        EventBridge              EventBridge
     (transformed)             (validation_failed)
```

## Concurrency Model

### Go Goroutines

Go's native concurrency model provides efficient parallel processing:

**Lambda Functions**:

- Each Lambda invocation runs in a single goroutine
- Use goroutines for parallel health checks
- Mutex for shared state (circuit breaker)

**Kafka Consumer**:

- Main goroutine: Message polling loop
- Worker goroutines: Message processing (configurable)
- Signal handling goroutine: Graceful shutdown
- Metrics server goroutine: Prometheus endpoint

**Advantages**:

- Lightweight (2KB stack per goroutine)
- M:N scheduling (efficient CPU utilization)
- Channel-based communication
- No callback hell (unlike Node.js)

**Trade-offs vs Rust**:

- Runtime overhead: ~2-3ms per goroutine creation
- GC pauses: ~1ms STW (though rare with Go 1.24.3+)
- Memory: ~5MB base overhead for runtime

## Error Handling

### Lambda Functions

```go
// Structured error handling
if err := processEvent(ctx, event); err != nil {
    logger.Error("processing failed", zap.Error(err))
    metrics.RecordError(functionName, err)
    
    // Send to DLQ
    if dlqErr := sendToDLQ(ctx, event, err); dlqErr != nil {
        logger.Error("DLQ send failed", zap.Error(dlqErr))
    }
    
    return fmt.Errorf("processing failed: %w", err)
}
```

### Kafka Consumer

```go
// Manual offset commit on success
if err := processor.Process(ctx, msg); err != nil {
    logger.Error("processing failed", zap.Error(err))
    // Don't commit offset - message will be reprocessed
    return err
}

// Commit only on success
consumer.CommitMessage(msg)
```

### Circuit Breaker

```go
// State machine: Closed → Open → Half-Open → Closed
err := circuitBreaker.Execute(func() error {
    return publisher.PublishEvent(ctx, event)
})

if err != nil {
    if errors.Is(err, ErrCircuitOpen) {
        // Circuit is open, fail fast
        return err
    }
    // Other error, record failure
}
```

## Monitoring & Observability

### Metrics

All components expose Prometheus metrics:

**Lambda Functions**: Embedded CloudWatch Metrics
**Kafka Consumer**: HTTP endpoint on :9090/metrics

### Logging

Structured logging with zap:

```go
logger.Info("processing event",
    zap.String("event_id", event.EventID),
    zap.String("event_type", event.EventType),
    zap.Duration("duration", duration),
)
```

### Tracing

- Trace ID propagation through event metadata
- Correlation ID for request tracking
- AWS X-Ray integration (optional)

## Performance Characteristics

### Lambda Cold Starts

**Go**: 100-150ms  
**Factors**:

- Runtime initialization: ~50ms
- Package imports: ~30ms
- AWS SDK initialization: ~20ms
- Secret retrieval: ~50ms (cached after first call)

**Optimization**:

- SnapStart (future): ~50ms reduction
- Provisioned concurrency: Eliminate cold starts
- Minimize imports: Only import what's needed

### Warm Execution

**P50**: 5-8ms  
**P99**: 8-12ms  
**P99.9**: 15-20ms

### Memory Usage

**Lambda**:

- Minimum: 80MB (small functions)
- Typical: 128-256MB
- Maximum: 512MB (event transformer)

**Kafka Consumer**:

- Base: 200MB
- Under load: 300-400MB
- Peak: 600MB (backlog processing)

### Throughput

**Lambda**:

- Concurrent executions: 1,000 (default)
- Burst: 3,000 (first minute)
- Sustained: 8,000-10,000 req/s per function

**Kafka Consumer**:

- Per pod: 8,000-12,000 msgs/sec
- 3 pods: 24,000-36,000 msgs/sec
- 10 pods (max): 80,000-120,000 msgs/sec

## Deployment Architecture

### Lambda Deployment

``` TXT
GitHub → GitHub Actions → Build (GOOS=linux GOARCH=arm64)
              ↓
         Package (zip)
              ↓
      Upload to S3 / SAM
              ↓
    Octopus Deploy → AWS Lambda
```

### EKS Deployment

``` TXT
GitHub → GitHub Actions → Build Docker Image
              ↓
         Push to GHCR
              ↓
      Update K8s Manifest
              ↓
    Octopus Deploy → kubectl apply
              ↓
         Rolling Update (EKS)
```

## Disaster Recovery

### Automatic Failover

1. Health checker detects unhealthy region
2. Circuit breaker opens for failed region
3. Traffic routes to healthy region
4. EventBridge rules update routing
5. Manual verification and testing
6. Gradual traffic shift back (circuit breaker half-open)

### Recovery Time Objective (RTO)

- Detection: 5 minutes (health check interval)
- Failover: 2 minutes (circuit breaker + routing)
- **Total RTO**: 7 minutes

### Recovery Point Objective (RPO)

- DynamoDB Global Tables: <1 second
- Kafka Cluster Linking: <5 seconds
- EventBridge: At-least-once delivery
- **Total RPO**: <10 seconds

## Security

### Lambda Functions ID

- IAM roles with least privilege
- VPC attachment for private resources
- Secrets Manager for sensitive data
- Environment variable encryption

### EKS ID

- IRSA (IAM Roles for Service Accounts)
- Network policies for pod-to-pod
- Security groups for node traffic
- Secrets stored in AWS Secrets Manager

### Kafka

- SASL/SSL authentication
- mTLS for inter-broker communication
- ACLs for topic access control
- Schema Registry authentication

## Cost Optimization

### Lambda

- ARM64 (Graviton2): 20% cheaper than x86
- Right-sized memory allocation
- EventBridge instead of polling
- Batch processing where possible

### EKS

- Spot instances for non-critical workloads
- Horizontal pod autoscaling
- Cluster autoscaler for node efficiency
- Reserved instances for baseline capacity

### Data Transfer

- Zstd compression: 60-70% size reduction
- Same-region data transfer: Free
- Cross-region: $0.02/GB (minimized with compression)

## Comparison: Go vs Rust

| Metric | Go | Rust | Winner |
| -------- | ---- | ---- | -------- |
| Cold Start | 100-150ms | 32ms | Rust |
| Warm Exec | 8-12ms | 7ms | Rust |
| Memory | 80-120MB | 48MB | Rust |
| Throughput | 8-10K/s | 12K/s | Rust |
| Dev Velocity | Fast | Slow | Go |
| Debugging | Easy | Hard | Go |
| Team Familiarity | High | Low | Go |
| Ecosystem | Mature | Growing | Go |

**Recommendation**: Use Go for faster development, Rust for maximum performance.

## Future Enhancements

1. **Observability**
   - AWS X-Ray distributed tracing
   - Custom Grafana dashboards
   - Anomaly detection with ML

2. **Performance**
   - Lambda SnapStart when available for Go
   - Custom runtime optimizations
   - Connection pooling improvements

3. **Scalability**
   - Multi-region Kafka cluster
   - Global DynamoDB with on-demand scaling
   - API Gateway with caching

4. **Reliability**
   - Chaos engineering with AWS FIS
   - Automated canary deployments
   - Blue-green deployments

---

**Last Updated**: 2026-02-09  
**Version**: 1.0  
**Author**: Principal Cloud Engineer / Principal Software Engineer
