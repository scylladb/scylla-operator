package crypto

import (
	"context"
	"sync"
	"testing"
)

// RunnableKeyGenerator is an interface satisfied by key generators that can be
// started in a background goroutine and closed when no longer needed.
// It is intentionally decoupled from the crypto package to avoid import cycles.
type RunnableKeyGenerator interface {
	Run(ctx context.Context)
	Close()
}

// StartKeyGenerator starts the generator's background goroutine and registers
// cleanup that cancels the context, waits for the goroutine, and closes the generator.
func StartKeyGenerator(t testing.TB, kg RunnableKeyGenerator) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		kg.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
		kg.Close()
	})
}
