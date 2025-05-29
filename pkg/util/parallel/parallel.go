// Copyright (C) 2017 ScyllaDB

package parallel

import (
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func ForEach(length int, f func(i int) error) error {
	errCh := make(chan error, length)
	defer close(errCh)

	for i := range length {
		go func(i int) {
			errCh <- f(i)
		}(i)
	}

	errs := make([]error, 0, length)
	for range length {
		errs = append(errs, <-errCh)
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
