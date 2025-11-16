# High-Performance MUS Cache Implementation

## Overview

Goverhaul uses MUS (Marshal/Unmarshal/Size) binary serialization for high-performance caching of lint violations. This implementation delivers exceptional speed, minimal memory usage, and excellent scalability for projects of all sizes.

## Why MUS?

MUS is a binary serialization format designed for performance-critical applications. Our implementation leverages:

- **Varint encoding** for compact representation
- **Manual serialization** for zero reflection overhead
- **Single allocation design** for minimal GC pressure
- **Linear scalability** with data size

## Architecture

### Core Components

#### MusCache Structure
```go
type MusCache struct {
    gCache *granular.Cache  // Underlying cache engine
    fs     afero.Fs         // Filesystem abstraction
}
```

The MusCache integrates with the granular cache system, providing a high-performance persistence layer optimized for lint violation data.

#### Manual Serialization

Custom serializers for `LintViolation` and `LintViolations` use:
- Varint encoding for integers and string lengths
- Direct byte copying for string data
- Pre-calculated size functions to minimize allocations

### Key Design Decisions

1. **Manual vs Generated Serialization**: Chose manual implementation for full control over memory allocations and encoding strategy

2. **Single Buffer Allocation**: Pre-calculate exact size needed and allocate once, avoiding incremental buffer growth

3. **Varint Encoding**: Uses variable-length encoding for integers, optimizing for common small values while supporting large ones

4. **String Optimization**: Length-prefixed strings with direct byte copying for optimal performance

## Performance Characteristics

### Speed
- **Encoding**: Sustained 2,700+ MB/s throughput across all dataset sizes
- **Decoding**: Sustained 1,000+ MB/s throughput with excellent consistency
- **Scalability**: Linear performance scaling from small to massive datasets

### Memory Efficiency
- **Single allocation** per encode/decode operation
- Minimal GC pressure even under high concurrency
- Memory usage scales linearly with data size

### Size Efficiency
- Compact binary format averaging ~195-203 bytes per violation
- Varint encoding minimizes size for common values
- No metadata overhead or type descriptors

### Real-World Performance

#### Small Projects (10-100 violations)
- Sub-millisecond cache operations
- Ideal for real-time IDE integration
- Negligible memory footprint

#### Medium Projects (1,000 violations)
- ~100 microseconds for encoding
- ~200 microseconds for decoding
- <200KB memory usage

#### Large Projects (10,000 violations)
- ~760 microseconds for encoding
- ~2 milliseconds for decoding
- ~2MB memory usage

#### Enterprise Scale (100,000 violations)
- ~7.4 milliseconds for encoding
- ~14.3 milliseconds for decoding
- ~20MB memory usage
- Perfect for monorepos and large codebases

## Usage

### Basic Usage

```go
// Create a new MUS cache
cache, err := goverhaul.NewMusCache(".goverhaul.cache", fs)
if err != nil {
    log.Fatal(err)
}

// Store violations for a file
violations := []goverhaul.LintViolation{
    {
        File:    "main.go",
        Import:  "unsafe",
        Rule:    "no-unsafe",
        Cause:   "unsafe package prohibited",
        Details: "Using unsafe can lead to undefined behavior",
    },
}

err = cache.AddFileWithViolations("main.go", violations)
if err != nil {
    log.Fatal(err)
}

// Retrieve cached violations
cached, err := cache.HasEntry("main.go")
if err == goverhaul.ErrEntryNotFound {
    // File not in cache
} else if err != nil {
    log.Fatal(err)
}

// Use cached violations
for _, v := range cached.Violations {
    fmt.Printf("Violation: %s in %s\n", v.Import, v.Rule)
}
```

### Integration with Granular

The MusCache seamlessly integrates with the granular caching system:

```go
// MusCache works with afero filesystem abstraction
fs := afero.NewOsFs()
cache, err := goverhaul.NewMusCache(".cache", fs)

// Or use in-memory filesystem for testing
memFs := afero.NewMemMapFs()
testCache, err := goverhaul.NewMusCache(".cache", memFs)
```

## Implementation Details

### Serialization Format

Each `LintViolation` is serialized as:
```
[File length (varint)][File bytes]
[Import length (varint)][Import bytes]
[Rule length (varint)][Rule bytes]
[Cause length (varint)][Cause bytes]
[Details length (varint)][Details bytes]
[Cached (1 byte)]
```

A `LintViolations` collection is serialized as:
```
[Number of violations (varint)]
[Violation 1]
[Violation 2]
...
[Violation N]
```

### Size Calculation

Before encoding, we calculate the exact buffer size needed:
```go
func sizeLintViolation(v *LintViolation) int {
    size := varint.SizeUint(uint(len(v.File))) + len(v.File)
    size += varint.SizeUint(uint(len(v.Import))) + len(v.Import)
    size += varint.SizeUint(uint(len(v.Rule))) + len(v.Rule)
    size += varint.SizeUint(uint(len(v.Cause))) + len(v.Cause)
    size += varint.SizeUint(uint(len(v.Details))) + len(v.Details)
    size += 1 // Cached bool
    return size
}
```

This allows a single allocation for the entire serialization buffer.

### Error Handling

The implementation includes comprehensive error handling:
- File system errors during cache operations
- Validation of deserialized data
- Clear error messages for troubleshooting

## Testing

### Test Coverage
- Comprehensive unit tests for all operations
- Edge case testing (empty data, Unicode, special characters)
- Concurrent access testing
- Integration tests with granular cache system

### Benchmarking
- Tests across 5 payload sizes (10 to 100,000 violations)
- Encode, decode, and round-trip benchmarks
- Memory allocation tracking
- Throughput measurements

## Best Practices

### When to Use MusCache

1. **CI/CD Pipelines**: Fast cache operations reduce build times
2. **IDE Integration**: Real-time performance for instant feedback
3. **Large Monorepos**: Scales efficiently to 10,000+ files
4. **High Concurrency**: Minimal GC pressure under parallel operations

### Configuration Tips

1. Use absolute paths for cache files to avoid path resolution overhead
2. Place cache on fast storage (SSD/NVMe) for optimal performance
3. Consider cache size limits for very large projects
4. Use in-memory filesystem for testing

## Conclusion

The MUS cache implementation provides exceptional performance for goverhaul's caching needs. With sustained 2,700+ MB/s encoding throughput, minimal memory allocations, and excellent scalability, it handles projects from small utilities to enterprise monorepos efficiently.

The single allocation design and manual serialization ensure predictable performance characteristics, making it ideal for both development workflows and production CI/CD pipelines.
