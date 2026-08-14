package utils

import "errors"

func RetryOnceOnError[T any](target error, operation func() (T, error)) (T, error) {
	value, err := operation()
	if err != nil && errors.Is(err, target) {
		return operation()
	}
	return value, err
}
