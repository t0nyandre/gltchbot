package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// RetryableFunc is a function that can be retried on transient failures.
// It should be idempotent.
type RetryableFunc func(ctx context.Context) error

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	MaxAttempts   int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	Jitter        bool
	RetryableErrs []string // PostgreSQL error codes to retry
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Jitter:      true,
		RetryableErrs: []string{
			pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.SQLClientUnableToEstablishSQLConnection,
			pgerrcode.SQLServerRejectedEstablishmentOfSQLConnection,
			pgerrcode.TransactionResolutionUnknown,
			pgerrcode.DeadlockDetected,
			// Add more retryable error codes as needed
		},
	}
}

// WithRetry executes the given function with retry logic.
// If the function returns a retryable error, it will be retried according to config.
// Returns the last error (either original or after max attempts).
func WithRetry(ctx context.Context, fn RetryableFunc, config RetryConfig) error {
	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		// Check if error is retryable
		if !isRetryableError(err, config.RetryableErrs) {
			return err
		}
		// If this was the last attempt, break
		if attempt == config.MaxAttempts {
			break
		}
		// Calculate delay with exponential backoff and optional jitter
		delay := exponentialBackoff(attempt, config.BaseDelay, config.MaxDelay, config.Jitter)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// continue to next attempt
		}
	}
	return lastErr
}

// isRetryableError checks if the error matches one of the retryable error codes.
func isRetryableError(err error, retryableErrs []string) bool {
	var pgErr *pgconn.PgError
	if !(errors.As(err, &pgErr)) {
		// Not a PostgreSQL error, check if it's a connection error etc.
		// For simplicity, we'll treat network/timeout errors as retryable
		// but for now we only retry PostgreSQL errors.
		return false
	}
	for _, code := range retryableErrs {
		if pgErr.Code == code {
			return true
		}
	}
	return false
}

// exponentialBackoff calculates delay for the given attempt.
func exponentialBackoff(attempt int, baseDelay, maxDelay time.Duration, jitter bool) time.Duration {
	delay := baseDelay * (1 << (attempt - 1)) // exponential: base * 2^(attempt-1)
	if delay > maxDelay {
		delay = maxDelay
	}
	if jitter {
		// Add +/- 25% jitter
		jitterFactor := 0.75 + 0.5*float64(time.Now().UnixNano()%1000)/1000.0 // random between 0.75 and 1.25
		delay = time.Duration(float64(delay) * jitterFactor)
	}
	return delay
}
