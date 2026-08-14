package utils

import (
	"errors"
	"fmt"
	"testing"
)

func TestRetryOnceOnError(t *testing.T) {
	target := errors.New("retryable")
	other := errors.New("other")
	second := fmt.Errorf("second retryable: %w", target)

	tests := []struct {
		name       string
		results    []string
		errors     []error
		wantResult string
		wantError  error
		wantCalls  int
	}{
		{
			name:       "returns first successful result",
			results:    []string{"first"},
			errors:     []error{nil},
			wantResult: "first",
			wantCalls:  1,
		},
		{
			name:       "retries a matching error",
			results:    []string{"discarded", "recovered"},
			errors:     []error{target, nil},
			wantResult: "recovered",
			wantCalls:  2,
		},
		{
			name:       "retries a wrapped matching error",
			results:    []string{"discarded", "recovered"},
			errors:     []error{fmt.Errorf("wrapped: %w", target), nil},
			wantResult: "recovered",
			wantCalls:  2,
		},
		{
			name:       "does not retry non matching error",
			results:    []string{"original"},
			errors:     []error{other},
			wantResult: "original",
			wantError:  other,
			wantCalls:  1,
		},
		{
			name:       "returns second matching failure",
			results:    []string{"first", "second"},
			errors:     []error{target, second},
			wantResult: "second",
			wantError:  second,
			wantCalls:  2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			operation := func() (string, error) {
				index := calls
				calls++
				return test.results[index], test.errors[index]
			}

			result, err := RetryOnceOnError(target, operation)

			if result != test.wantResult {
				t.Fatalf("result = %q, want %q", result, test.wantResult)
			}
			if !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
			if calls != test.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, test.wantCalls)
			}
		})
	}
}

func TestRetryOnceOnErrorDoesNotRetrySuccessfulOperation_whenTargetIsNil(t *testing.T) {
	calls := 0
	operation := func() (string, error) {
		calls++
		return "success", nil
	}

	result, err := RetryOnceOnError(nil, operation)

	if result != "success" || err != nil {
		t.Fatalf("result = %q, error = %v, want successful first result", result, err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
