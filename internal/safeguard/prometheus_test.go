package safeguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusQuerier(t *testing.T) {
	q, err := NewPrometheusQuerier("http://localhost:9090")
	require.NoError(t, err)
	assert.NotNil(t, q)

	q, err = NewPrometheusQuerier("ftp://bad")
	require.Error(t, err)
	assert.Nil(t, q)

	q, err = NewPrometheusQuerier("http://")
	require.Error(t, err)
	assert.Nil(t, q)
}

func TestPrometheusQuerier_InstantQuery(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantValue  float64
		wantErr    bool
	}{
		{
			name:       "success with scalar value",
			response:   `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1234567890,"1.5"]}]}}`,
			statusCode: 200,
			wantValue:  1.5,
		},
		{
			name:       "empty result",
			response:   `{"status":"success","data":{"resultType":"vector","result":[]}}`,
			statusCode: 200,
			wantErr:    true,
		},
		{
			name:       "prometheus error response",
			response:   `{"status":"error","errorType":"bad_data","error":"invalid query"}`,
			statusCode: 200,
			wantErr:    true,
		},
		{
			name:       "server error",
			response:   "internal server error",
			statusCode: 500,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			q, err := NewPrometheusQuerier(srv.URL)
			require.NoError(t, err)

			got, err := q.InstantQuery(context.Background(), "up")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

func TestPrometheusQuerier_RangeQuery(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "multiple data points",
			response:   `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1000,"1.0"],[1010,"2.0"],[1020,"3.0"]]}]}}`,
			statusCode: 200,
			wantCount:  3,
		},
		{
			name:       "empty result",
			response:   `{"status":"success","data":{"resultType":"matrix","result":[]}}`,
			statusCode: 200,
			wantCount:  0,
		},
		{
			name:       "server error",
			response:   `internal server error`,
			statusCode: 500,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			q, err := NewPrometheusQuerier(srv.URL)
			require.NoError(t, err)

			got, err := q.RangeQuery(context.Background(), "up", time.Unix(1000, 0), time.Unix(1020, 0), 10*time.Second)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, got, tt.wantCount)
		})
	}
}
