package safeguard

import (
	"context"
	"time"
)

// DataPoint is a single timestamped metric value
type DataPoint struct {
	Time  time.Time
	Value float64
}

// MetricsQuerier abstracts the metrics backend
type MetricsQuerier interface {
	// InstantQuery executes a PromQL query and returns the scalar result.
	// "What is the error rate RIGHT NOW?"
	InstantQuery(ctx context.Context, query string) (float64, error)

	// RangeQuery executes a PromQL range query and returns a time series.
	// "What was the error rate every 30s for the last hour?"
	RangeQuery(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]DataPoint, error)
}
