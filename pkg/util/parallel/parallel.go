// Copyright (C) 2017 ScyllaDB

package parallel

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func ForEach(length int, f func(i int) error) error {
	errCh := make(chan error, length)
	defer close(errCh)

	for i := 0; i < length; i++ {
		go func(i int) {
			errCh <- f(i)
		}(i)
	}

	errs := make([]error, 0, length)
	for i := 0; i < length; i++ {
		errs = append(errs, <-errCh)
	}

	return utilerrors.NewAggregate(errs)
}
