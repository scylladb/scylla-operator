package helpers

func Must[T any](r T, err error) T {
	if err != nil {
		panic(err)
	}

	return r
}
