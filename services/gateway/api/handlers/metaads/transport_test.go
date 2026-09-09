package metaads

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetchAllUsesBearerTokenAndStripsPaginationToken(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer system-token", request.Header.Get("Authorization"))
		require.Empty(t, request.URL.Query().Get("access_token"))
		requestNumber := requests.Add(1)
		response.Header().Set("Content-Type", "application/json")
		if requestNumber == 1 {
			_, _ = fmt.Fprintf(response, `{"data":[{"id":"1","name":"A"}],"paging":{"next":%q}}`, serverURL(request)+"?after=next&access_token=must-not-propagate")
			return
		}
		_, _ = response.Write([]byte(`{"data":[{"id":"2","name":"B"}]}`))
	}))
	defer server.Close()

	client := newGraphClient(server.URL, "v26.0", "1", "system-token", time.Second)
	items, err := client.fetchCampaigns(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, int32(2), requests.Load())
}

func TestGraphClientRetriesRateLimit(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 1 {
			response.WriteHeader(http.StatusTooManyRequests)
			_, _ = response.Write([]byte(`{"error":{"message":"rate limited","code":4,"is_transient":true}}`))
			return
		}
		_, _ = response.Write([]byte(`{"id":"act_1","name":"Account"}`))
	}))
	defer server.Close()

	client := newGraphClient(server.URL, "v26.0", "1", "system-token", 2*time.Second)
	client.maxRetries = 1
	_, err := client.fetchAccount(context.Background())

	require.NoError(t, err)
	require.Equal(t, int32(2), requests.Load())
}

func TestGraphClientDoesNotRetryAuthErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"error":{"message":"invalid token system-token","code":190}}`))
	}))
	defer server.Close()

	client := newGraphClient(server.URL, "v26.0", "1", "system-token", time.Second)
	_, err := client.fetchAccount(context.Background())

	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "system-token"))
	require.Equal(t, int32(1), requests.Load())
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host + request.URL.Path
}
