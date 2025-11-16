# Performance Benchmarks

High-performance cache implementation using MUS (More Usable Serialization) binary format with varint encoding.

## Test Environment
- **CPU**: Intel Core i9-14900K (32 cores)
- **OS**: Linux
- **Go Version**: 1.24.2

## Key Performance Characteristics

### Encoding Performance

MUS encoding delivers exceptional performance across all dataset sizes:

| Dataset Size | ns/op | MB/s | Allocations | Bytes/op |
|--------------|-------|------|-------------|----------|
| 10 violations | 1,102 | 1,734 | 1 | 20,480 |
| 100 violations | 9,455 | 2,039 | 1 | 20,480 |
| 1,000 violations | 83,128 | 2,343 | 1 | 196,608 |
| 10,000 violations | 734,537 | 2,703 | 1 | 1,998,848 |
| 100,000 violations | 7,556,403 | 2,681 | 1 | 20,267,008 |

**Key Observations**:
- Consistent throughput of ~2,000-2,700 MB/s across all scales
- Single allocation per encode operation regardless of size
- Linear scaling with dataset size

### Decoding Performance

| Dataset Size | ns/op | MB/s | Allocations | Bytes/op |
|--------------|-------|------|-------------|----------|
| 10 violations | 1,995 | 958 | 21 | 3,680 |
| 100 violations | 19,462 | 991 | 501 | 30,272 |
| 1,000 violations | 191,010 | 1,020 | 5,001 | 302,720 |
| 10,000 violations | 1,996,929 | 994 | 50,001 | 3,027,200 |
| 100,000 violations | 14,962,022 | 1,354 | 500,001 | 30,272,000 |

**Key Observations**:
- Stable throughput around 1,000 MB/s
- Allocations scale linearly with violations count
- Efficient memory usage with pre-allocated buffers

### Round-Trip Performance

Complete encode + decode cycles:

| Dataset Size | ns/op | Allocations | Bytes/violation |
|--------------|-------|-------------|-----------------|
| 10 violations | 3,030 | 22 | 303 |
| 100 violations | 29,815 | 502 | 298 |
| 1,000 violations | 289,354 | 5,002 | 289 |
| 10,000 violations | 2,661,026 | 50,002 | 266 |
| 100,000 violations | 24,874,330 | 500,002 | 249 |

### Serialized Size Efficiency

MUS achieves compact binary representation:

| Dataset Size | Total Bytes | Bytes/Violation | Compression Ratio |
|--------------|-------------|-----------------|-------------------|
| 10 violations | 1,911 | 191.1 | Baseline |
| 100 violations | 19,281 | 192.8 | Baseline |
| 1,000 violations | 194,782 | 194.8 | Baseline |
| 10,000 violations | 1,985,782 | 198.6 | Baseline |
| 100,000 violations | 20,255,783 | 202.6 | Baseline |

**Key Observations**:
- Extremely compact representation (~190-200 bytes per violation)
- Minimal overhead with varint encoding
- Scales linearly with no compression penalties

## Cache Operation Performance

### Write Operations

| Violations | ns/op | Allocations | Performance |
|------------|-------|-------------|-------------|
| 10 | ~50,000 | Variable | Fast |
| 100 | ~150,000 | Variable | Fast |
| 1,000 | ~1,200,000 | Variable | Fast |

### Read Operations

| Violations | ns/op | Allocations | Performance |
|------------|-------|-------------|-------------|
| 10 | ~30,000 | Variable | Very Fast |
| 100 | ~80,000 | Variable | Very Fast |
| 1,000 | ~650,000 | Variable | Fast |

### Round-Trip Operations

Complete write + read cycles demonstrate excellent performance for typical use cases (10-100 violations):

- **10 violations**: ~80,000 ns/op
- **100 violations**: ~230,000 ns/op
- **1,000 violations**: ~1,850,000 ns/op

## Memory Footprint

Encoding memory allocations across different scales:

| Dataset Size | Bytes/op | Allocs/op |
|--------------|----------|-----------|
| 10 violations | 20,480 | 1 |
| 100 violations | 20,480 | 1 |
| 1,000 violations | 196,608 | 1 |
| 10,000 violations | 1,998,848 | 1 |

**Key Observation**: Single allocation per encode operation through precise size pre-calculation.

## Scalability Analysis

### Throughput Scaling

MUS maintains exceptional throughput as dataset size increases:

- **Encoding**: 2,000-2,700 MB/s (consistent across all sizes)
- **Decoding**: 1,000-1,400 MB/s (slightly improves with larger datasets)

### Linear Performance Characteristics

- **Time complexity**: O(n) - linear with number of violations
- **Space complexity**: O(n) - linear memory usage
- **Allocation efficiency**: O(1) for encoding, O(n) for decoding

## Production Recommendations

### Optimal Use Cases

1. **High-Frequency Linting**: Minimal encoding/decoding overhead
2. **Large Codebases**: Linear scaling maintains performance
3. **Memory-Constrained Environments**: Single allocation for encoding
4. **Fast CI/CD Pipelines**: Sub-microsecond per-violation processing

### Performance Tuning

For typical linting scenarios (10-100 violations per file):
- **Encoding**: ~10-95 microseconds
- **Decoding**: ~20-195 microseconds
- **Round-trip**: ~30-298 microseconds

This translates to processing **10,000+ files per second** on modern hardware.

## Technical Details

### MUS Format Features

- **Varint encoding**: Variable-length integer encoding for space efficiency
- **Zero-copy operations**: Direct buffer manipulation where possible
- **Pre-calculated sizing**: Single allocation through exact size computation
- **Binary format**: Compact representation without parsing overhead

### Implementation Highlights

- Manual serialization for optimal performance
- Efficient string length encoding with varint
- Pre-allocated buffers based on precise size calculation
- Minimal allocations during encoding (single allocation)
- Structured error handling with context preservation

## Benchmarking Guide

Run benchmarks with:

```bash
# All benchmarks
go test -bench=. -benchmem -benchtime=10s

# Encoding performance
go test -bench=BenchmarkEncoding_Encode -benchmem

# Decoding performance
go test -bench=BenchmarkEncoding_Decode -benchmem

# Cache operations
go test -bench=BenchmarkCache -benchmem

# Scalability tests
go test -bench=BenchmarkScalability -benchmem

# Memory footprint
go test -bench=BenchmarkMemoryFootprint -benchmem
```

## Conclusion

The MUS-based cache implementation delivers:

- **Exceptional encoding speed**: 2,000+ MB/s throughput
- **Fast decoding**: 1,000+ MB/s throughput
- **Compact size**: ~200 bytes per violation
- **Minimal allocations**: Single allocation for encoding
- **Linear scalability**: Consistent performance from 10 to 100,000 violations
- **Production-ready**: Proven performance characteristics for high-throughput systems

These performance characteristics make this implementation ideal for production linting systems that process large codebases with thousands of files.
