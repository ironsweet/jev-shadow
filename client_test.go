package jev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleResponse = `
{
	"model": "jev-style-2b-decision-v3",
	"answers": {
		"intent_covered": {
			"type": "noul",
			"noul": 0.8849556062190528
		}
	},
	"usage": {
		"input_tokens": 108,
		"state_tokens": 64,
		"output_tokens": 0
	},
	"latency_ms": 2069.2,
	"timing": {
		"total_ms": 2069.2
	},
	"backend": "torch-cuda"
}`

func writeSample(w http.ResponseWriter) {
	_, _ = w.Write([]byte(sampleResponse))
}

func fakeJevServer(t *testing.T, handler func(http.ResponseWriter, *SystemOneRequest) bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			http.Error(w, "not found", http.StatusNotFound)
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}

		var soReq SystemOneRequest
		err := json.NewDecoder(r.Body).Decode(&soReq)
		if err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		if handler(w, &soReq) {
			return
		}
		writeSample(w)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_Judge_Success(t *testing.T) {
	reqCh := make(chan SystemOneRequest, 1)
	srv := fakeJevServer(t, func(w http.ResponseWriter, req *SystemOneRequest) bool {
		reqCh <- *req
		return false
	})

	cc := NewClient(srv.URL, 5*time.Second)
	resp, err := cc.Judge(t.Context(),
		"User intention: x\nTool calls: y",
		"intent_covered",
		"Intent is covered?")
	require.NoError(t, err)

	soRequest := <-reqCh
	require.NotNil(t, soRequest)
	_, ok := soRequest.Questions["intent_covered"]
	assert.True(t, ok)
	assert.NotEmpty(t, soRequest.State)

	require.NotNil(t, resp)
	prob, ok := resp.NoulProbability("intent_covered")
	require.True(t, ok)
	assert.Equal(t, 0.8849556062190528, prob)
	assert.Equal(t, 2069.2, resp.LatencyMs)
}

func TestClient_Judge_Error(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *SystemOneRequest) bool
		assert  func(t *testing.T, err error)
	}{
		{
			name: "non-200",
			handler: func(w http.ResponseWriter, r *SystemOneRequest) bool {
				http.Error(w, "boom", http.StatusUnprocessableEntity)
				return true
			},
			assert: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "422")
			},
		},
		{
			name: "malformed-json",
			handler: func(w http.ResponseWriter, r *SystemOneRequest) bool {
				_, _ = w.Write([]byte(`{not json`))
				return true
			},
			assert: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "invalid character")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeJevServer(t, tc.handler)
			cc := NewClient(srv.URL, 2*time.Second)
			_, err := cc.Judge(t.Context(), "s", "q", "i")
			tc.assert(t, err)
		})
	}
}

func TestClient_Judge_UnexpectedAnswerShape(t *testing.T) {
	srv := fakeJevServer(t, func(w http.ResponseWriter, r *SystemOneRequest) bool {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"other_question": map[string]any{"type": "noul", "noul": 0.5},
				"wrong_type":     map[string]any{"type": "choice"},
			},
		}))
		return true
	})
	cc := NewClient(srv.URL, 2*time.Second)
	resp, err := cc.Judge(t.Context(), "s", "intent_covered", "i")
	require.NoError(t, err)
	_, ok := resp.NoulProbability("intent_covered")
	assert.False(t, ok)
	_, ok = resp.NoulProbability("wrong_type")
	assert.False(t, ok)
}

func TestClient_Judge_ContextTimeout(t *testing.T) {
	srv := fakeJevServer(t, func(w http.ResponseWriter, r *SystemOneRequest) bool {
		select {
		case <-t.Context().Done():
		case <-time.After(10 * time.Second):
		}
		return false
	})

	cc := NewClient(srv.URL, 30*time.Second)
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := cc.Judge(ctx, "s", "q", "i")
	elapsed := time.Since(start)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Greater(t, elapsed.Milliseconds(), int64(400), "expected timeout close to ctx timeout")
	assert.Less(t, elapsed.Milliseconds(), int64(1000))
}
