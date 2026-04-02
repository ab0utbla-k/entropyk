package safeguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var _ AlertChecker = (*AlertmanagerChecker)(nil)

// AlertmanagerChecker queries an Alertmanager instance over HTTP.
type AlertmanagerChecker struct {
	baseURL *url.URL
	client  *http.Client
}

// NewAlertmanagerChecker creates a checker pointing at the given Alertmanager URL.
// Example: NewAlertmanagerChecker("http://alertmanager.monitoring.svc:9093")
func NewAlertmanagerChecker(baseURL string) (*AlertmanagerChecker, error) {
	u, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &AlertmanagerChecker{
		baseURL: u,
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

type amAlert struct {
	Labels map[string]string `json:"labels"`
	Status amAlertStatus     `json:"status"`
}

type amAlertStatus struct {
	State string `json:"state"`
}

// CheckAlerts returns firing alerts matching all given label matchers.
func (a *AlertmanagerChecker) CheckAlerts(ctx context.Context, labelMatchers map[string]string) ([]Alert, error) {
	u := a.baseURL.JoinPath("api", "v2", "alerts")
	u.RawQuery = url.Values{"active": {"true"}}.Encode()

	body, err := getJSON(ctx, a.client, u)
	if err != nil {
		return nil, err
	}

	var alerts []amAlert
	if err := json.Unmarshal(body, &alerts); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	res := make([]Alert, 0)
	for _, alert := range alerts {
		if alert.Status.State != "active" { // defense-in-depth: API filters with ?active=true
			continue
		}

		matched := true
		for k, v := range labelMatchers {
			if alert.Labels[k] != v {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}

		res = append(res, Alert{
			Name:   alert.Labels["alertname"],
			Labels: alert.Labels,
		})

	}

	return res, nil
}
