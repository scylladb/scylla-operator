# EventBus

A generic, thread-safe event processing package for Go based on channels. EventBus allows multiple subscribers to receive events from a single input channel, with optional event filtering and configurable buffer sizes.

## Features

- **Generic Type Support**: Works with any type using Go generics
- **Thread-Safe**: Safe for concurrent use by multiple goroutines
- **Event Filtering**: Subscribers can filter events using custom filter functions
- **Configurable Buffers**: Each subscriber can have its own channel buffer size
- **Non-Blocking Distribution**: Slow subscribers don't block event distribution
- **Context Support**: Automatic unsubscription when context is cancelled
- **Zero External Dependencies**: Pure Go implementation

## Installation

```bash
go get github.com/gocql/gocql/eventbus
```

## Quick Start

```go
package main

import (
	"fmt"
	"github.com/gocql/gocql/internal/eventbus"
)

func main() {
	// Create a new EventBus for integer events
	eb := eventbus.New[int](10) // input buffer size of 10

	// Start processing events
	eb.Start()
	defer eb.Stop()

	// Subscribe to all events
	allSub, _ := eb.Subscribe("subscriber1", 10, nil)

	// Subscribe with a filter (only even numbers)
	evenFilter := func(n int) bool { return n%2 == 0 }
	evenSub, _ := eb.Subscribe("subscriber2", 10, evenFilter)

	// Send events
	go func() {
		for i := 1; i <= 5; i++ {
			eb.PublishEvent(i)
		}
	}()

	// Receive events
	for event := range allSub.Events() {
		fmt.Println("All:", event)
		// Process event...
		if event == 5 {
			break
		}
	}

	// Clean up
	allSub.Stop()
	evenSub.Stop()
}
```

## API Overview

### Creating an EventBus

```go
eb := eventbus.New[T](inputChanSize int) *EventBus[T]
```

Creates a new EventBus for type `T` with specified input channel buffer size.

### Starting and Stopping

```go
err := eb.Start()  // Start processing events
err := eb.Stop()   // Stop processing and close all subscriber channels
```

### Subscribing

```go
// Subscribe without filter
sub, err := eb.Subscribe("subscriber-name", chanSize, nil)

// Subscribe with filter
filter := func(event T) bool { return /* condition */ }
sub, err := eb.Subscribe("subscriber-name", chanSize, filter)

// Subscribe with context (auto-unsubscribes when context is cancelled)
sub, err := eb.SubscribeWithContext(ctx, "subscriber-name", chanSize, filter)

// Access events from subscriber
events := sub.Events()  // Returns <-chan T
```

### Unsubscribing

```go
// Using the Subscriber instance (recommended)
err := sub.Stop()

// Or using the EventBus directly
err := eb.Unsubscribe("subscriber-name")
```

### Sending Events

```go
eb.PublishEvent(event)
```

### Getting Information

```go
count := eb.SubscriberCount()
str := eb.String() // Debug string representation
```

## Examples

### Basic Usage

```go
eb := eventbus.New[string](10)
eb.Start()
defer eb.Stop()

sub, _ := eb.Subscribe("logger", 10, nil)
defer sub.Stop()

go func() {
    eb.PublishEvent("Hello")
    eb.PublishEvent("World")
}()

for msg := range sub.Events() {
    fmt.Println(msg)
}
```

### With Event Filtering

```go
type LogEvent struct {
    Level   string
    Message string
}

eb := eventbus.New[LogEvent](10)
eb.Start()
defer eb.Stop()

// Subscribe to errors only
errorFilter := func(e LogEvent) bool { return e.Level == "ERROR" }
errorSub, _ := eb.Subscribe("error-handler", 10, errorFilter)
defer errorSub.Stop()

// Subscribe to all events
allSub, _ := eb.Subscribe("all-handler", 10, nil)
defer allSub.Stop()

// Access events
for event := range errorSub.Events() {
    // Handle error events only
}
```

### With Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

eb := eventbus.New[int](10)
eb.Start()
defer eb.Stop()

// Automatically unsubscribes after 5 seconds
sub, _ := eb.SubscribeWithContext(ctx, "temp", 10, nil)

for {
    select {
    case event := <-sub.Events():
        // Process event
    case <-ctx.Done():
        return
    }
}
```

## Design Decisions

### Non-Blocking Distribution

EventBus uses non-blocking sends to subscriber channels. If a subscriber's channel is full, the event is dropped for that subscriber only, ensuring that slow subscribers don't block the event bus or other subscribers.

### Channel Closure

- Subscriber channels are closed when `Subscriber.Stop()` or `EventBus.Unsubscribe()` is called
- All subscriber channels are closed when `EventBus.Stop()` is called
- When using `SubscribeWithContext()`, channels are closed when the context is cancelled

### Subscriber API

The `Subscribe()` and `SubscribeWithContext()` methods return a `Subscriber` instance that provides:
- `Events()` - Returns the receive-only channel for events
- `Stop()` - Unsubscribes and closes the channel

### Thread Safety

All public methods are thread-safe and can be called concurrently from multiple goroutines. The EventBus uses read-write mutexes to minimize contention during event distribution.

## Performance Considerations

- **Buffer Sizes**: Choose appropriate buffer sizes based on your event rate and processing speed
- **Filter Functions**: Keep filter functions fast; they're called for every event-subscriber pair
- **Slow Subscribers**: Slow subscribers with small buffers will drop events; increase buffer size or process events asynchronously

## Testing

Run unit tests:

```bash
go test -tags unit -v ./eventbus
```

Run benchmarks:

```bash
go test -tags unit -bench=. ./eventbus
```

## License

Licensed under the Apache License, Version 2.0. See LICENSE file for details.
