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
	prob, latency, err := cc.Judge(t.Context(), "goal", "do something")
	require.NoError(t, err)

	soRequest := <-reqCh
	_, ok := soRequest.Questions["intent_covered"]
	assert.True(t, ok)
	assert.Contains(t, soRequest.State, "goal")
	assert.Contains(t, soRequest.State, "do something")

	assert.Equal(t, 0.8849556062190528, prob)
	assert.Equal(t, 2069.2, latency)
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
				assert.ErrorContains(t, err, "decode")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeJevServer(t, tc.handler)
			cc := NewClient(srv.URL, 2*time.Second)
			_, _, err := cc.Judge(t.Context(), "goal", "do something")
			tc.assert(t, err)
		})
	}
}

func TestClient_Judge_UnexpectedAnswerShape(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *SystemOneRequest) bool
	}{
		{
			name: "missing-intent_covered",
			handler: func(w http.ResponseWriter, r *SystemOneRequest) bool {
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"answers": map[string]any{
						"other_question": map[string]any{"type": "noul", "noul": 0.5},
					},
				}))
				return true
			},
		},
		{
			name: "wrong-type-intent_covered",
			handler: func(w http.ResponseWriter, r *SystemOneRequest) bool {
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"answers": map[string]any{
						"intent_covered": map[string]any{"type": "choice"},
					},
				}))
				return true
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeJevServer(t, tc.handler)
			cc := NewClient(srv.URL, 2*time.Second)
			_, _, err := cc.Judge(t.Context(), "goal", "do something")
			assert.ErrorContains(t, err, "intent_covered")
		})
	}
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
	_, _, err := cc.Judge(ctx, "goal", "do something")
	elapsed := time.Since(start)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Greater(t, elapsed.Milliseconds(), int64(400), "expected timeout close to ctx timeout")
	assert.Less(t, elapsed.Milliseconds(), int64(1000))
}
