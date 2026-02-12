package tempuscache

import (
	"testing"
	"time"
)

/*
BenchmarkSet measures the performance of the Set() operation.

PURPOSE

Benchmarks are used to evaluate:
- Execution time per operation (ns/op)
- Memory allocations (when run with -benchmem)
- Throughput under repeated execution

This benchmark focuses specifically on measuring the cost of:

1. Expiration timestamp calculation
2. Mutex Lock()/Unlock() overhead
3. Map write operation
4. Struct assignment

HOW GO BENCHMARKS WORK

The testing framework dynamically determines b.N,
the number of iterations required to produce stable results.

The loop:

    for i := 0; i < b.N; i++

is automatically scaled by the Go runtime to measure
accurate per-operation performance.

WHAT THIS BENCHMARK REPRESENTS

- Ideal scenario where the same key is overwritten repeatedly.
- Map size does not grow significantly.
- Measures core write-path performance.

For more realistic benchmarks, variations could include:
- Using unique keys (map growth behavior)
- Parallel benchmarking (mutex contention testing)
- Measuring allocations using: go test -bench=. -benchmem

This benchmark provides insight into the raw performance
characteristics of the cache’s write operation.
*/

func BenchmarkSet(b *testing.B) {
	cache := New()

	for i := 0; i < b.N; i++ {
		cache.Set("key", "value", 5*time.Second)
	}
}
