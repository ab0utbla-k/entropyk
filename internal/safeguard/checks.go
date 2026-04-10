package safeguard

import (
	"context"
	"fmt"
	"strconv"

	temperv1alpha1 "github.com/ab0utbla-k/temper/api/v1alpha1"
)

func CheckAlertsFiring(ctx context.Context, labels map[string]string, checker AlertChecker) (string, error) {
	if len(labels) == 0 {
		return "", nil
	}

	alerts, err := checker.CheckAlerts(ctx, labels)
	if err != nil {
		return "", err
	}

	if len(alerts) > 0 {
		return fmt.Sprintf("Critical alert: %s", alerts[0].Name), nil
	}

	return "", nil
}

func CheckSLOBreach(ctx context.Context, slo *temperv1alpha1.SLOProtection, querier MetricsQuerier) (string, error) {
	if slo.Threshold == nil {
		return "SLO protection configured without threshold", nil
	}

	threshold, err := strconv.ParseFloat(*slo.Threshold, 64)
	if err != nil {
		return fmt.Sprintf("Invalid SLO threshold %q: %v", *slo.Threshold, err), nil
	}

	for _, query := range slo.Queries {
		val, err := querier.InstantQuery(ctx, query.Query)
		if err != nil {
			return "", err
		}
		if val >= threshold {
			return fmt.Sprintf("SLO breached: %s = %f (threshold %f)", query.Name, val, threshold), nil
		}
	}
	return "", nil
}
