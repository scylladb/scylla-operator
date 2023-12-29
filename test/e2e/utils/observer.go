// Copyright (c) 2023 ScyllaDB.

package utils

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"k8s.io/apimachinery/pkg/util/wait"
	watchutils "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
)

type ObserverEvent[T kubeinterfaces.ObjectInterface] struct {
	Action watchutils.EventType
	Obj    T
}

type ObjectObserver[T kubeinterfaces.ObjectInterface] struct {
	Events []ObserverEvent[T]

	lw cache.ListerWatcher

	errChan chan error
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func (o *ObjectObserver[T]) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	o.cancel = cancel

	_, informer, watcher, done := watch.NewIndexerInformerWatcher(o.lw, *new(T))

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("unable to sync caches: %w", ctx.Err())
	}

	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer func() { <-done }()
		defer watcher.Stop()

		_, err := watch.UntilWithoutRetry(ctx, watcher, func(e watchutils.Event) (bool, error) {
			o.Events = append(o.Events, ObserverEvent[T]{
				Action: e.Type,
				Obj:    e.Object.DeepCopyObject().(T),
			})
			return false, nil
		})

		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) && wait.Interrupted(err) {
				o.errChan <- nil
				return
			}
			o.errChan <- err
		}

		o.errChan <- nil
	}()

	return nil
}

func (o *ObjectObserver[T]) Stop() ([]ObserverEvent[T], error) {
	o.cancel()

	o.wg.Wait()
	err := <-o.errChan

	return o.Events, err
}

func ObserveObjects[T kubeinterfaces.ObjectInterface](lw cache.ListerWatcher) ObjectObserver[T] {
	return ObjectObserver[T]{
		lw:      lw,
		errChan: make(chan error, 1),
	}
}
