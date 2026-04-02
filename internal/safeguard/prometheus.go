package safeguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type promResponse struct {
	Status    string   `json:"status"`
	Data      promData `json:"data"`
	ErrorType string   `json:"errorType"`
	Error     string   `json:"error"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promResult `json:"result"`
}

type promResult struct {
	Value  []json.RawMessage   `json:"value"`  // [timestamp, "value"]
	Values [][]json.RawMessage `json:"values"` // [[ts, "val"], ...]
}

var _ MetricsQuerier = (*PrometheusQuerier)(nil)

// PrometheusQuerier queries a PromQL-compatible metrics backend over HTTP.
type PrometheusQuerier struct {
	baseURL *url.URL
	client  *http.Client
}

// NewPrometheusQuerier creates a querier pointing at the given Prometheus URL.
// Example: NewPrometheusQuerier("http://prometheus.monitoring.svc:9090")
func NewPrometheusQuerier(baseURL string) (*PrometheusQuerier, error) {
	u, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &PrometheusQuerier{
		baseURL: u,
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// InstantQuery executes a PromQL instant query and returns the scalar result.
func (p *PrometheusQuerier) InstantQuery(ctx context.Context, query string) (float64, error) {
	u := p.baseURL.JoinPath("api", "v1", "query")
	u.RawQuery = url.Values{"query": {query}}.Encode()

	body, err := getJSON(ctx, p.client, u)
	if err != nil {
		return 0, err
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	if pr.Status != "success" {
		return 0, fmt.Errorf("prometheus: %s", pr.Error)
	}

	if len(pr.Data.Result) == 0 {
		return 0, fmt.Errorf("empty result")
	}

	if len(pr.Data.Result) > 1 {
		return 0, fmt.Errorf("expected single series, got %d", len(pr.Data.Result))
	}

	if len(pr.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("malformed value tuple")
	}

	var val string
	if err := json.Unmarshal(pr.Data.Result[0].Value[1], &val); err != nil {
		return 0, fmt.Errorf("unmarshal value: %w", err)
	}

	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}
	return f, nil
}

// RangeQuery executes a PromQL range query and returns a time series.
func (p *PrometheusQuerier) RangeQuery(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]DataPoint, error) {
	u := p.baseURL.JoinPath("api", "v1", "query_range")
	params := url.Values{
		"query": {query},
		"start": {fmt.Sprintf("%d", start.Unix())},
		"end":   {fmt.Sprintf("%d", end.Unix())},
		"step":  {fmt.Sprintf("%.0fs", step.Seconds())},
	}
	u.RawQuery = params.Encode()
	body, err := getJSON(ctx, p.client, u)
	if err != nil {
		return nil, err
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus: %s", pr.Error)
	}

	if len(pr.Data.Result) == 0 {
		return nil, nil
	}

	if len(pr.Data.Result) > 1 {
		return nil, fmt.Errorf("expected single series, got %d", len(pr.Data.Result))
	}

	res := make([]DataPoint, 0, len(pr.Data.Result[0].Values))
	for _, val := range pr.Data.Result[0].Values {
		if len(val) < 2 {
			return nil, fmt.Errorf("malformed value tuple")
		}

		var t float64
		if err := json.Unmarshal(val[0], &t); err != nil {
			return nil, fmt.Errorf("unmarshal timestamp: %w", err)
		}

		var v string
		if err := json.Unmarshal(val[1], &v); err != nil {
			return nil, fmt.Errorf("unmarshal value: %w", err)
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("parse float: %w", err)
		}

		res = append(res, DataPoint{Value: f, Time: time.Unix(int64(t), 0)})
	}

	return res, nil
}
