package safeguard

import "context"

// Alert represents a single firing alert.
type Alert struct {
	Name   string
	Labels map[string]string
}

// AlertChecker abstracts the alert source.
type AlertChecker interface {
	// CheckAlerts returns firing alerts matching all given label matchers.
	// Example: CheckAlerts(ctx, map[string]string{"severity": "critical"})
	// returns all critical alerts currently firing.
	CheckAlerts(ctx context.Context, labelMatchers map[string]string) ([]Alert, error)
}
