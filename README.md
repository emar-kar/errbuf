# errbuf

[![Go Reference](https://pkg.go.dev/badge/github.com/emar-kar/errbuf.svg)](https://pkg.go.dev/github.com/emar-kar/errbuf)
[![Go Report Card](https://goreportcard.com/badge/github.com/emar-kar/errbuf)](https://goreportcard.com/report/github.com/emar-kar/errbuf)

`errbuf` is optimized, thread-safe error buffer for Go. It allows you to concurrently aggregate multiple errors and format them efficiently with **zero allocation overhead** on the fast paths.

## Installation

```bash
go get github.com/emar-kar/errbuf
```

## Usage

### Basic Usage

Create an error buffer, safely add errors from multiple goroutines, and evaluate the final result.

```go
package main

import (
	"errors"
	"fmt"
	"sync"
	
	"github.com/emar-kar/errbuf"
)

func main() {
	buf := errbuf.New()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Safely add errors concurrently.
			buf.Add(fmt.Errorf("error from worker %d", id))
		}(i)
	}
	
	wg.Wait()

	// Conditionally check if any errors were recorded.
	if err := buf.Err(); err != nil {
		fmt.Printf("Encountered errors: %v\n", err)
	}
}
```

### Memory Optimized Batching

If you run repeating batch tasks, use `Clear()` to instantly empty the buffer while keeping the underlying array capacity initialized. This guarantees exactly 0 dynamic append allocations, perfectly maximizing runtime speed.

```go
buf := errbuf.New()
buf.Grow(100) // Optionally pre-allocate exactly 100 slots

for batch := 0; batch < 1000; batch++ {
	errs := processBatch(batch)
	buf.Add(errs...)
	
	if err := buf.Err(); err != nil {
		log.Println("Batch failed:", err)
	}
	
	buf.Clear() // O(1) allocation clear.
}
```

### Custom Formatting

`BufferedError` natively implements `fmt.Formatter` so you can format errors cleanly.

```go
buf := errbuf.New()
buf.Add(errors.New("connection reset"), errors.New("timeout"))

// Standard line separated by semicolons.
fmt.Printf("%s\n", buf) 
// Output: "connection reset; timeout"

// Multiline formatting.
fmt.Printf("%v\n", buf)
// Output: 
// connection reset
// timeout

// Full inspection.
fmt.Printf("%+v\n", buf)
// Output: BufferedError{errors:[connection reset timeout]}
```

### `errors.Is` and `errors.As` support

You can query deeply into the buffer without needing to manually unroll it.

```go
var ErrNotFound = errors.New("not found")

buf := errbuf.New()
buf.Add(errors.New("database disconnected"), ErrNotFound)

// Checks all errors inside the buffer for the target
if errors.Is(buf, ErrNotFound) {
	fmt.Println("Item was not found!")
}
```

## Benchmarks

The package is deeply tuned for performance against internal micro-benchmarks:

```text
BenchmarkBufferedErrorAddSlice1000-10               3894      30914 ns/op       12293 B/op         1 allocs/op
BenchmarkBufferedErrorAddSlice1000All-10            8650      14219 ns/op       12294 B/op         1 allocs/op
BenchmarkBufferedErrorAdd1-10                    2511114         47.29 ns/op        0 B/op         0 allocs/op
BenchmarkBufferedErrorAddParallel-10              950526        144.6 ns/op       104 B/op         0 allocs/op
BenchmarkBuffereErrorMultiLine-10                   3517      33364 ns/op       12346 B/op         1 allocs/op
```

## License

[MIT License](LICENSE)