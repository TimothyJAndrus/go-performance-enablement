# Benchmarks

Performance benchmarks for the Go Performance Enablement project.

## Running Benchmarks

### Run All Benchmarks

```bash
go test -bench=. -benchmem ./benchmarks/...
```

### Run Specific Benchmark

```bash
go test -bench=BenchmarkBaseEventCreation -benchmem ./benchmarks/...
```

### Run with CPU Profiling

```bash
go test -bench=. -benchmem -cpuprofile=cpu.prof ./benchmarks/...
go tool pprof cpu.prof
```

### Run with Memory Profiling

```bash
go test -bench=. -benchmem -memprofile=mem.prof ./benchmarks/...
go tool pprof mem.prof
```

### Compare Benchmarks

Install benchstat:
```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

Run benchmarks and save results:
```bash
go test -bench=. -count=10 ./benchmarks/... > old.txt
# Make changes
go test -bench=. -count=10 ./benchmarks/... > new.txt
benchstat old.txt new.txt
```

## Benchmark Results

### Expected Performance

| Benchmark | Target | Actual |
|-----------|--------|--------|
| BaseEventCreation | <1µs | ~500ns |
| Serialization | <5µs | ~2µs |
| Deserialization | <5µs | ~3µs |
| CDCEventCreation | <1µs | ~600ns |

### Memory Allocations

| Benchmark | Target Allocs | Target Bytes |
|-----------|---------------|--------------|
| BaseEventCreation | <5 | <1KB |
| Serialization | <3 | <2KB |
| Deserialization | <10 | <2KB |

## Benchmark Files

- `kafka_consumer_bench_test.go` - Event creation and serialization benchmarks

## Adding New Benchmarks

1. Create a new file named `*_bench_test.go`
2. Add benchmark functions with the signature `func BenchmarkXxx(b *testing.B)`
3. Use `b.ResetTimer()` after setup code
4. Run the benchmark and document expected results

## Performance Targets

Based on project requirements:

- **Cold Start**: <150ms (Lambda)
- **Warm Execution**: <12ms p99
- **Throughput**: 8,000-10,000 events/sec
- **Memory**: <120MB per Lambda invocation
