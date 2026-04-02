package safeguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAlertmanagerChecker(t *testing.T) {
	q, err := NewAlertmanagerChecker("http://localhost:9090")
	require.NoError(t, err)
	assert.NotNil(t, q)

	q, err = NewAlertmanagerChecker("ftp://bad")
	require.Error(t, err)
	assert.Nil(t, q)

	q, err = NewAlertmanagerChecker("http://")
	require.Error(t, err)
	assert.Nil(t, q)
}

func TestAlertmanagerChecker_CheckAlerts(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		matchers   map[string]string
		wantName   string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "no alerts",
			response:   `[]`,
			statusCode: 200,
			matchers:   map[string]string{"severity": "critical"},
		},
		{
			name:       "no matching labels",
			response:   `[{"labels":{"alertname":"Foo","severity":"warning"},"status":{"state":"active"}}]`,
			statusCode: 200,
			matchers:   map[string]string{"severity": "critical"},
		},
		{
			name:       "matching alert",
			response:   `[{"labels":{"alertname":"HighErrorRate","severity":"critical"},"status":{"state":"active"}}]`,
			statusCode: 200,
			matchers:   map[string]string{"severity": "critical"},
			wantCount:  1,
			wantName:   "HighErrorRate",
		},
		{
			name:       "suppressed alert filtered out",
			response:   `[{"labels":{"alertname":"Foo","severity":"critical"},"status":{"state":"suppressed"}}]`,
			statusCode: 200,
			matchers:   map[string]string{"severity": "critical"},
		},
		{
			name:       "partial label match not enough",
			response:   `[{"labels":{"alertname":"Foo","severity":"critical"},"status":{"state":"active"}}]`,
			statusCode: 200,
			matchers:   map[string]string{"severity": "critical", "team": "platform"},
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

			c, err := NewAlertmanagerChecker(srv.URL)
			require.NoError(t, err)

			got, err := c.CheckAlerts(context.Background(), tt.matchers)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, got, tt.wantCount)
			if tt.wantName != "" {
				assert.Equal(t, tt.wantName, got[0].Name)
			}
		})
	}
}
