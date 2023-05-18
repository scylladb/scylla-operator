// Copyright (C) 2023 ScyllaDB

package itemgenerator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type Generator[T any] struct {
	name              string
	generateFunc      func() (*T, error)
	unlimitedBuffer   chan *T
	rateLimitedBuffer chan *T
	rateLimiterDelay  time.Duration
}

func NewGenerator[T any](name string, min, max int, delay time.Duration, generateFunc func() (*T, error)) (*Generator[T], error) {
	if min < 1 {
		return nil, fmt.Errorf("min (%d) can't be lower then 1", min)
	}

	if max < 1 {
		return nil, fmt.Errorf("max (%d) can't be lower then 1", max)
	}

	if max < min {
		return nil, fmt.Errorf("max (%d) can't be lower then min (%d)", max, min)
	}

	if generateFunc == nil {
		return nil, errors.New("generateFunc can't be nil")
	}

	return &Generator[T]{
		name:         name,
		generateFunc: generateFunc,
		// There is always one extra cached item waiting to be written to the channel.
		unlimitedBuffer:   make(chan *T, min-1),
		rateLimitedBuffer: make(chan *T, max-min),
		rateLimiterDelay:  delay,
	}, nil
}

func (g *Generator[T]) runWorker(ctx context.Context, ch chan<- *T) error {
	startTime := time.Now()
	klog.V(4).InfoS("Generating item", "Name", g.name, "BufferCapacity", cap(ch), "BufferSize", len(ch))

	item, err := g.generateFunc()
	if err != nil {
		return fmt.Errorf("generator %q: can't generate item: %w", g.name, err)
	}
	klog.V(4).InfoS(
		"Generating item finished",
		"Name", g.name,
		"BufferCapacity", cap(ch),
		"BufferSize", len(ch),
		"Duration", time.Now().Sub(startTime),
	)

	select {
	case <-ctx.Done():
		return ctx.Err()

	case ch <- item:
		return nil
	}
}

func (g *Generator[T]) runUnlimitedWorker(ctx context.Context) {
	klog.V(4).InfoS("Generator is starting unlimited worker", "Name", g.name)

	errDelay := 1 * time.Second
	errorTicker := time.NewTicker(errDelay)
	defer errorTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.V(4).InfoS("Generator unlimited worker canceled.", "Name", g.name, "ContextError", ctx.Err())
			return

		default:
			err := g.runWorker(ctx, g.unlimitedBuffer)
			if err != nil {
				klog.ErrorS(err, "Generator unlimited worker encountered error", "Name", g.name)

				errorTicker.Reset(errDelay)

				select {
				case <-ctx.Done():
					break
				case <-errorTicker.C:
					break
				}

				continue
			}
		}
	}
}

func (g *Generator[T]) runRateLimitedWorker(ctx context.Context) {
	klog.V(4).InfoS("Generator is starting rate limited worker", "Name", g.name)

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		err := g.runWorker(ctx, g.rateLimitedBuffer)
		if err != nil {
			klog.ErrorS(err, "Generator rate limited worker encountered error", "Name", g.name)
			time.Sleep(time.Second)
			return
		}
	}, g.rateLimiterDelay)
}

func (g *Generator[T]) Run(ctx context.Context) {
	defer func() {
		klog.V(4).InfoS("Generator workers have finished.", "Name", g.name, "ContextError", ctx.Err())
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		g.runUnlimitedWorker(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		g.runRateLimitedWorker(ctx)
	}()
}

func (g *Generator[T]) Close() {
	close(g.unlimitedBuffer)
	close(g.rateLimitedBuffer)
}

func (g *Generator[T]) GetItem(ctx context.Context) (*T, error) {
	select {
	case item := <-g.rateLimitedBuffer:
		return item, nil
	case item := <-g.unlimitedBuffer:
		return item, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
